package vegeta

import (
	"crypto/tls"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

type DiyAttacker struct {
	dialer     *net.Dialer
	client     fasthttp.Client
	stopch     chan struct{}
	workers    uint64
	maxWorkers uint64
	maxBody    int64
	seqmu      sync.Mutex
	seq        uint64
	began      time.Time
}

func NewDiyAttacker(opts ...func(*DiyAttacker)) *DiyAttacker {
	a := &DiyAttacker{
		stopch:     make(chan struct{}),
		workers:    DefaultWorkers,
		maxWorkers: DefaultMaxWorkers,
		maxBody:    DefaultMaxBody,
		began:      time.Now(),
	}

	a.client = fasthttp.Client{
		MaxConnsPerHost: 1000,
		ReadTimeout:     DefaultTimeout,
		WriteTimeout:    DefaultTimeout,
		Dial: func(addr string) (net.Conn, error) {
			return fasthttp.DialTimeout(addr, 60*time.Second)
		},
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Workers returns a functional option which sets the initial number of workers
// an Attacker uses to hit its targets. More workers may be spawned dynamically
// to sustain the requested rate in the face of slow responses and errors.
func DiyWorkers(n uint64) func(*DiyAttacker) {
	return func(a *DiyAttacker) { a.workers = n }
}

// MaxWorkers returns a functional option which sets the maximum number of workers
// an Attacker can use to hit its targets.
func DiyMaxWorkers(n uint64) func(*DiyAttacker) {
	return func(a *DiyAttacker) { a.maxWorkers = n }
}

type DiyTargeter func(*Target) (string, bool, error)

func (a *DiyAttacker) DiyAttack(tr DiyTargeter, p Pacer, du time.Duration, loopCounts int, debug bool) <-chan *Result {
	var wg sync.WaitGroup

	workers := a.workers
	if workers > a.maxWorkers {
		workers = a.maxWorkers
	}

	results := make(chan *Result)
	ticks := make(chan struct{})
	for i := uint64(0); i < workers; i++ {
		wg.Add(1)
		go a.diyAttack(tr, &wg, ticks, results, debug)
	}

	go func() {
		defer close(results)

		//该函数必须先将fasthttp.Client的m和ms进行导出变为M和Ms，还需要将fasthttp.HostClient的conns进行导出变为Conns
		//所有goroutine请求结束后，关闭存活的所有net.Conn，以避免向server发送[RST,ACK]
		//defer函数为后声明的先执行。所有该释放连接的函数必须在wg.Wait()之前声明。
		//以保证所有持有net.Conn的goroutine运行结束。
		defer func() {
			for _, v := range a.client.M {
				for index := range v.Conns {
					_ = v.Conns[index].C.Close()
				}
			}
			for _, v := range a.client.Ms {
				for index := range v.Conns {
					_ = v.Conns[index].C.Close()
				}
			}
		}()

		defer wg.Wait()
		defer close(ticks)

		began, count := time.Now(), uint64(0)
		for {
			elapsed := time.Since(began)
			//运行时间或者运作次数，完成一个即整体结束。
			if (du > 0 && elapsed > du) || (loopCounts > 0 && count+1 > uint64(loopCounts)) {
				return
			}

			wait, stop := p.Pace(elapsed, count)
			if stop {
				return
			}

			time.Sleep(wait)

			if workers < a.maxWorkers {
				select {
				case ticks <- struct{}{}:
					count++
					continue
				case <-a.stopch:
					return
				default:
					// all workers are blocked. start one more and try again
					workers++
					wg.Add(1)
					go a.diyAttack(tr, &wg, ticks, results, debug)
				}
			}

			select {
			case ticks <- struct{}{}:
				count++
			case <-a.stopch:
				return
			}
		}
	}()

	return results
}

func (a *DiyAttacker) diyAttack(tr DiyTargeter, workers *sync.WaitGroup, ticks <-chan struct{}, results chan<- *Result, debug bool) {
	defer workers.Done()
	for range ticks {
		results <- a.diyHit(tr, debug)
	}
}

func (a *DiyAttacker) diyHit(tr DiyTargeter, debug bool) *Result {
	var (
		res = Result{Attack: DefaultName}
		tgt Target
		err error
	)

	a.seqmu.Lock()
	res.Timestamp = a.began.Add(time.Since(a.began))
	res.Seq = a.seq
	a.seq++
	a.seqmu.Unlock()

	defer func() {
		res.Latency = time.Since(res.Timestamp)
		if err != nil {
			res.Error = err.Error()
		}
	}()

	name, disableKeepAlive, err := tr(&tgt)
	if err != nil {
		a.Stop()
		return &res
	}

	res.Attack = name

	res.Method = tgt.Method
	res.URL = tgt.URL

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(tgt.URL)
	req.Header.SetMethod(tgt.Method)
	req.SetBody(tgt.Body)

	for k, v := range tgt.Header {
		req.Header.Add(k, v[0])
	}

	if disableKeepAlive {
		//禁用keepAlive
		//req.Header.Add(fasthttp.HeaderConnection, "close")
		req.SetConnectionClose()
	}

	req.Header.Add("X-Vegeta-Attack", DefaultName)
	req.Header.Add("X-Vegeta-Seq", strconv.FormatUint(res.Seq, 10))

	err = a.client.Do(req, resp)
	if err != nil {
		return &res
	}

	respBody := resp.Body()

	res.BytesIn = uint64(len(respBody))
	if debug {
		res.ReqBody = string(tgt.Body)
		res.RspBody = string(respBody)
		res.Headers = nil

	}

	if req.Header.ContentLength() != -1 {
		res.BytesOut = uint64(req.Header.ContentLength())
	}

	if res.Code = uint16(resp.StatusCode()); res.Code < 200 || res.Code >= 400 {
		res.Error = strconv.Itoa(resp.StatusCode())
	}

	return &res
}

func (a *DiyAttacker) Stop() {
	select {
	case <-a.stopch:
		return
	default:
		close(a.stopch)
	}
}

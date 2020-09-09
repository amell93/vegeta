package vegeta

import (
	"io"
	"io/ioutil"
	"strconv"
	"sync"
	"time"
)

type DiyTargeter func(*Target) (string, error)

func (a *Attacker) DiyAttack(tr DiyTargeter, p Pacer, du time.Duration, debug bool) <-chan *Result {
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
		defer wg.Wait()
		defer close(ticks)

		began, count := time.Now(), uint64(0)
		for {
			elapsed := time.Since(began)
			if du > 0 && elapsed > du {
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

func (a *Attacker) diyAttack(tr DiyTargeter, workers *sync.WaitGroup, ticks <-chan struct{}, results chan<- *Result, debug bool) {
	defer workers.Done()
	for range ticks {
		results <- a.diyHit(tr, debug)
	}
}

func (a *Attacker) diyHit(tr DiyTargeter, debug bool) *Result {
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

	name, err := tr(&tgt)
	if err != nil {
		a.Stop()
		return &res
	}

	res.Attack = name

	res.Method = tgt.Method
	res.URL = tgt.URL

	req, err := tgt.Request()
	if err != nil {
		return &res
	}
	req.Header.Set("X-Vegeta-Attack", name)

	req.Header.Set("X-Vegeta-Seq", strconv.FormatUint(res.Seq, 10))

	if a.chunked {
		req.TransferEncoding = append(req.TransferEncoding, "chunked")
	}

	r, err := a.client.Do(req)
	if err != nil {
		return &res
	}
	defer r.Body.Close()

	if !debug {
		length, err := io.Copy(ioutil.Discard, r.Body)
		if err != nil {
			return &res
		}
		res.BytesIn = uint64(length)
	} else {

		if res.Body, err = ioutil.ReadAll(r.Body); err != nil {
			return &res
		}

		res.ReqBody = string(tgt.Body)
		res.RspBody = string(res.Body)

		res.BytesIn = uint64(len(res.Body))

		res.Headers = r.Header
	}

	if req.ContentLength != -1 {
		res.BytesOut = uint64(req.ContentLength)
	}

	if res.Code = uint16(r.StatusCode); res.Code < 200 || res.Code >= 400 {
		res.Error = r.Status
	}


	return &res
}

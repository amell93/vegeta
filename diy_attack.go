package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

func diyAttackCmd() command {
	fs := flag.NewFlagSet("vegeta diyAttack", flag.ExitOnError)
	opts := &diyAttackOpts{}

	fs.StringVar(&opts.script, "s", "", "the script file")
	fs.Uint64Var(&opts.every, "every", 0, "report interval")
	fs.StringVar(&opts.output, "output", "stdout", "output file of the summary result, default is stdout")

	return command{fs, func(args []string) error {
		fs.Parse(args)
		return diyAttack(opts)
	}}
}

// attackOpts aggregates the attack function command options
type diyAttackOpts struct {
	script string
	every  uint64
	output string
}

// attack validates the attack arguments, sets up the
// required resources, launches the attack and writes the results
func diyAttack(opts *diyAttackOpts) (err error) {

	if opts.script == "" {
		panic("script file must exits")
	}

	var tr vegeta.DiyTargeter
	tr, sc := vegeta.NewWeightTargeter(opts.script)

	if sc.Rate == 0 && sc.MaxWorkers == vegeta.DefaultMaxWorkers {
		return fmt.Errorf("rate=0 requires setting maxWorkers")
	}

	if sc.Duration < 1 {
		return fmt.Errorf("duration must be integer	and need greater than 0")
	}

	//output the summary result
	out, err := file(opts.output, true)
	if err != nil {
		return fmt.Errorf("error opening %s: %s", opts.output, err)
	}
	defer out.Close()

	//record the result of each request

	var (
		resultFile *os.File
		enc        vegeta.Encoder
	)

	if sc.ResultFile != "" {
		resultFile, err = file(sc.ResultFile, true)
		if err != nil {
			return fmt.Errorf("error opening %s: %s", sc.ResultFile, err)
		}
		enc = vegeta.NewJSONEncoder(resultFile)
		defer resultFile.Close()
	}

	needRecord := resultFile != nil

	atk := vegeta.NewDiyAttacker(
		vegeta.DiyWorkers(sc.Workers),
		vegeta.DiyMaxWorkers(sc.MaxWorkers),
		//vegeta.KeepAlive(true),
	/*		vegeta.Redirects(opts.redirects),
			vegeta.Timeout(opts.timeout),
			vegeta.LocalAddr(*opts.laddr.IPAddr),
			vegeta.TLSConfig(tlsc),
			vegeta.Workers(opts.workers),
			vegeta.MaxWorkers(opts.maxWorkers),
			vegeta.KeepAlive(opts.keepalive),
			vegeta.Connections(opts.connections),
			vegeta.MaxConnections(opts.maxConnections),
			vegeta.HTTP2(opts.http2),
			vegeta.H2C(opts.h2c),
			vegeta.MaxBody(opts.maxBody),
			vegeta.UnixSocket(opts.unixSocket),
			vegeta.ProxyHeader(proxyHdr),
			vegeta.ChunkedBody(opts.chunked),*/
	)

	res := atk.DiyAttack(
		tr, vegeta.ConstantPacer{Freq: sc.Rate, Per: time.Second},
		time.Duration(sc.Duration)*time.Second, sc.Debug)
	//enc := vegeta.NewCSVEncoder(out)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	var (
		rep    vegeta.Reporter
		report vegeta.Report
	)

	m := vegeta.NewDiyMetrics()
	rep, report = vegeta.NewDiyTextReporter(m), m

	rc, _ := report.(vegeta.Closer)

	var ticks <-chan time.Time
	if opts.every > 0 {
		ticker := time.NewTicker(time.Duration(opts.every) * time.Second)
		defer ticker.Stop()
		ticks = ticker.C
	}
run:
	for {
		select {
		case <-sig:
			atk.Stop()
			break run
		case <-ticks:
			if err = clear(out); err != nil {
				return err
			} else if err = writeReport(rep, rc, out); err != nil {
				return err
			}
		case r, ok := <-res:
			if !ok {
				break run
			}

			if needRecord {
				if err = enc.Encode(r); err != nil {
					return err
				}
			}

			report.Add(r)
		}
	}

	clear(out)

	return writeReport(rep, rc, out)
}

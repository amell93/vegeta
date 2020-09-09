package vegeta

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
)

type DiyMetrics struct {
	Data map[string]*Metrics
}

func NewDiyMetrics() *DiyMetrics {
	return &DiyMetrics{
		Data: make(map[string]*Metrics),
	}
}

func (d *DiyMetrics) Add(r *Result) {
	if _, ok := d.Data[r.Attack]; !ok {
		d.Data[r.Attack] = &Metrics{}
	}
	d.Data[r.Attack].Add(r)
}

func (d *DiyMetrics) Close() {
	for _, v := range d.Data {
		v.Close()
	}
}

// NewTextReporter returns a Reporter that writes out Metrics as aligned,
// formatted text.
func NewDiyTextReporter(d *DiyMetrics) Reporter {
	const fmtstr = "Name:\t%s\nRequests\t[total, rate, throughput]\t%d, %.2f, %.2f\n" +
		"Duration\t[total, attack, wait]\t%s, %s, %s\n" +
		"Latencies\t[min, mean, 50, 90, 95, 99, max]\t%s, %s, %s, %s, %s, %s, %s\n" +
		"Bytes In\t[total, mean]\t%d, %.2f\n" +
		"Bytes Out\t[total, mean]\t%d, %.2f\n" +
		"Success\t[ratio]\t%.2f%%\n" +
		"Status Codes\t[code:count]\t"

	return func(w io.Writer) (err error) {
		tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', tabwriter.StripEscape)
		length := len(d.Data)
		i := 0
		for k, m := range d.Data {
			if _, err = fmt.Fprintf(tw, fmtstr,
				k, m.Requests, m.Rate, m.Throughput,
				round(m.Duration+m.Wait),
				round(m.Duration),
				round(m.Wait),
				round(m.Latencies.Min),
				round(m.Latencies.Mean),
				round(m.Latencies.P50),
				round(m.Latencies.P90),
				round(m.Latencies.P95),
				round(m.Latencies.P99),
				round(m.Latencies.Max),
				m.BytesIn.Total, m.BytesIn.Mean,
				m.BytesOut.Total, m.BytesOut.Mean,
				m.Success*100,
			); err != nil {
				return err
			}

			codes := make([]string, 0, len(m.StatusCodes))
			for code := range m.StatusCodes {
				codes = append(codes, code)
			}

			sort.Strings(codes)

			for _, code := range codes {
				count := m.StatusCodes[code]
				if _, err = fmt.Fprintf(tw, "%s:%d  ", code, count); err != nil {
					return err
				}
			}

			if _, err = fmt.Fprintln(tw, "\nError Set:"); err != nil {
				return err
			}

			for _, e := range m.Errors {
				if _, err = fmt.Fprintln(tw, e); err != nil {
					return err
				}
			}
			if i != length-1 {
				if _, err = fmt.Fprintln(tw); err != nil {
					return err
				}
			}
			i++
		}

		return tw.Flush()
	}
}

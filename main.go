package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type timedReport struct {
	ts time.Time
	Report
}

type state_t struct {
	sync.Mutex
	reports []timedReport
}

var state = state_t{
	reports: []timedReport{},
}

func (st *state_t) Add(rpt Report) {
	st.Lock()
	defer st.Unlock()
	st.reports = append(st.reports, timedReport{ts: time.Now(), Report: rpt})
	st.cleanup()
}

func (st *state_t) cleanup() {
	limit := time.Now().Add(-300 * time.Second)

	i := 0
	for i < len(st.reports) && st.reports[i].ts.Before(limit) {
		i++
	}

	if i > 0 {
		copy(st.reports, st.reports[i:])
		st.reports = st.reports[:len(st.reports)-i]
	}
}

func (st *state_t) Coalesce() (avg Report, last timedReport) {
	st.Lock()
	defer st.Unlock()
	st.cleanup()
	var n float64

	for _, r := range st.reports {
		last = r
		if r.PowerState > avg.PowerState {
			avg.PowerState = r.PowerState
		}
		avg.PSVolts += r.PSVolts
		avg.BatteryVolts += r.BatteryVolts
		avg.ChargeAmps += r.ChargeAmps
		avg.SolarVolts += r.SolarVolts
		avg.ChargePower += r.ChargePower
		avg.TempF += r.TempF
		n += 1
	}
	avg.PSVolts /= n
	avg.BatteryVolts /= n
	avg.ChargeAmps /= n
	avg.SolarVolts /= n
	avg.ChargePower /= n
	avg.TempF /= n
	return
}

func cvt(value, offset, mult float64) int {
	v := int((value-offset)/mult + 0.5)
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return v
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		avg, last := state.Coalesce()

		if last.ts.IsZero() {
			w.WriteHeader(http.StatusGone)
			w.Write([]byte(""))
			return
		}

		charging := 0
		if avg.PowerState == MPPT {
			charging = 1
		}

		fmt.Fprintf(
			w,
			"%03d %03d %03d %03d %03d %d0000000\n",
			cvt(avg.TempF, 0, 0.5),
			cvt(avg.BatteryVolts, 10, 0.05),
			cvt(avg.ChargeAmps, 0, 0.05),
			cvt(avg.SolarVolts, 10, 0.05),
			cvt(avg.ChargePower, 0, 1),
			charging,
		)

		fmt.Fprintf(
			w,
			"# Temp = %.0f F (avg %.2f F)\n# Bat = %.2f V (avg %.2f V)\n# Chg = %.2f A (avg %.2f A)\n# Sol = %.2f V (avg %.2f V)\n# Pow = %.2f W (avg %.2f W)\n",
			last.TempF, avg.TempF,
			last.BatteryVolts, avg.BatteryVolts,
			last.ChargeAmps, avg.ChargeAmps,
			last.SolarVolts, avg.SolarVolts,
			last.ChargePower, avg.ChargePower,
		)
	})

	go http.ListenAndServe(":9099", nil)

	p, err := NewPWRGate("/dev/pwrgate")
	if err != nil {
		panic(err)
	}
	defer p.Close()
	for r := range p.Reports {
		state.Add(r)
	}
}

package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tarm/serial"
)

type PowerState int

const (
	Unknown PowerState = iota
	PSOff
	BadTemp
	MPPT
	Trickle
	Charged
)

type Report struct {
	PowerState   PowerState
	PSVolts      float64
	BatteryVolts float64
	ChargeAmps   float64
	SolarVolts   float64
	ChargePower  float64
	TempF        float64
}

type PWRGate struct {
	ser     *serial.Port
	Reports chan Report
	exit    chan struct{}
}

func (p *PWRGate) readLoop() {
	defer func() {
		close(p.Reports)
	}()

	var retried bool
	reconfigure = true
	var buf string
	lineReg := regexp.MustCompile(`^\s*(.*?)\s+(PS[^S].*)\r\n`)
	pairReg := regexp.MustCompile(`(\S[^=]*)=\s*([^=]*[^= ])(\s+|$)`)
	batReg := regexp.MustCompile(`^(\S+)V,\s+(\S+)A$`)

	for {
		select {
		case <-p.exit:
			return
		default:
		}
		var bytes [1]byte
		n, _ := p.ser.Read(bytes[:])
		if n == 0 {
			if retried {
				return
			} else {
				retried = true
				fmt.Fprintln(os.Stderr, "read timeout, trying pressing enter")
				serialWrite(p.ser, "", "\r")
				continue
			}
		}

		buf = buf + string(bytes[:n])
		if len(buf) > 256 {
			buf = buf[len(buf)-256:]
		}

		for {
			//			fmt.Fprintln(os.Stderr, []byte(buf))
			//			fmt.Fprintln(os.Stderr, buf)
			if loc := lineReg.FindStringIndex(buf); loc != nil {
				line := buf[loc[0]:loc[1]]
				buf = buf[loc[1]:]

				m := lineReg.FindStringSubmatch(line)
				if m == nil {
					panic("huh?")
				}

				powerStatus := m[1]
				pairs := pairReg.FindAllStringSubmatch(m[2], -1)
				vars := map[string]string{}

				for _, pair := range pairs {
					vars[pair[1]] = pair[2]
				}

				r := Report{}
				switch powerStatus {
				case "MPPT":
					r.PowerState = MPPT
				case "PS Off":
					r.PowerState = PSOff
				case "Trickle":
					r.PowerState = Trickle
				case "Charged":
					r.PowerState = Charged
				case "Bad temp":
					r.PowerState = BadTemp
				default:
					r.PowerState = Unknown
				}

				var err error

				if psvolts, ok := vars["PS"]; ok {
					r.PSVolts, err = strconv.ParseFloat(strings.TrimSuffix(psvolts, "V"), 64)
					if err != nil {
						fmt.Println("PSVolts", psvolts)
						continue
					}
				}

				if bat, ok := vars["Bat"]; ok {
					m := batReg.FindStringSubmatch(bat)
					if m == nil {
						fmt.Println("Bat", bat)
						continue
					}

					r.BatteryVolts, err = strconv.ParseFloat(m[1], 64)
					if err != nil {
						fmt.Println("BatteryVolts", m[1])
						continue
					}
					r.ChargeAmps, err = strconv.ParseFloat(m[2], 64)
					if err != nil {
						fmt.Println("ChargeAmps", m[4])
						continue
					}
					r.ChargePower = r.ChargeAmps * r.BatteryVolts
				}

				if sol, ok := vars["Sol"]; ok {
					r.SolarVolts, err = strconv.ParseFloat(strings.TrimSuffix(sol, "V"), 64)
					if err != nil {
						fmt.Println("SolarVolts", sol)
						continue
					}
				}

				if temp, ok := vars["Temp"]; ok {
					r.TempF, err = strconv.ParseFloat(temp, 64)
					if err != nil {
						fmt.Println("TempF", temp)
						continue
					}
				}

				p.Reports <- r

				if reconfigure {
					fmt.Fprintln(p.ser, "x")
					reconfigure = false
				}
			} else {
				var ok bool
				var msg string
				ok, buf, msg = respond(p.ser, buf)
				if ok {
					if msg != "" {
						fmt.Fprintln(os.Stderr, msg)
					}
				} else {
					break
				}
			}
		}
	}
}

func NewPWRGate(path string) (*PWRGate, error) {
	sc := &serial.Config{
		Name:        path,
		Baud:        115200,
		Size:        8,
		Parity:      serial.ParityNone,
		StopBits:    1,
		ReadTimeout: 5 * time.Second,
	}
	ser, err := serial.OpenPort(sc)
	if err != nil {
		return nil, err
	}

	p := &PWRGate{
		Reports: make(chan Report, 10),
		ser:     ser,
	}

	go p.readLoop()

	return p, nil
}

func (p *PWRGate) Close() {
	close(p.exit)
}

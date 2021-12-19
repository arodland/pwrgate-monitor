package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
)

var reconfigure bool
var pressedS bool

func serialWrite(ser io.ReadWriter, buf string, line string) string {
	b := []byte(line)
	var tmp [1]byte

	for i := range b {
		time.Sleep(10 * time.Millisecond)
		ser.Write(b[i : i+1])
		if b[i] == 13 {
			time.Sleep(100 * time.Millisecond)
		}

		if len(buf) > 0 {
			buf = buf[1:]
		} else {
			ser.Read(tmp[:])
			if b[i] != tmp[0] {
				fmt.Fprintf(os.Stderr, "wrote %d, got back %d\n", b[i], tmp[0])
			}
		}
	}
	return buf
}

func serialWriteln(ser io.ReadWriter, buf string, line string) string {
	return serialWrite(ser, buf, line+"\r")
}

var pressSMsg = regexp.MustCompile(`Press S to Review/Edit Charge settings\r\n`)
var jumpersMsg = regexp.MustCompile(`Jumpers: .*\r\n`)
var becauseJumperMsg = regexp.MustCompile(`Because jumper is.*\r\n`)
var batteryMsg = regexp.MustCompile(`Battery: .*>: `)
var batteryAltMsg = regexp.MustCompile(`5-Other: .*>: `)
var resetToDefaultMsg = regexp.MustCompile(`Reset to default .*>\? `)
var maxVoltsMsg = regexp.MustCompile(`Max charge voltage .*>: `)
var maxAmpsMsg = regexp.MustCompile(`Max charge current .*>: `)
var minAmpsMsg = regexp.MustCompile(`Min charge current .*>: `)
var rechargeMsg = regexp.MustCompile(`Recharge voltage .*>: `)
var maxMinMsg = regexp.MustCompile(`Max charge \(min.*>: `)
var retryAfterMsg = regexp.MustCompile(`Retry after .*>: `)
var minSupplyMsg = regexp.MustCompile(`Min supply voltage .*>: `)
var trickleAmpsMsg = regexp.MustCompile(`Trickle current .*>: `)
var trickleVoltsMsg = regexp.MustCompile(`Trickle voltage .*>: `)
var minTempMsg = regexp.MustCompile(`Lowest Charge Temp .*>: `)
var maxTempMsg = regexp.MustCompile(`Highest Charge Temp .*>: `)
var tempAdjustMsg = regexp.MustCompile(`Use temp to adjust .*>\? `)

func respond(ser io.ReadWriter, buf string) (bool, string, string) {
	for len(buf) > 0 && (buf[0] == 10 || buf[0] == 13) {
		buf = buf[1:]
	}

	if buf == "" {
		return false, buf, ""
	}

	if m := pressSMsg.FindStringIndex(buf); m != nil {
		fmt.Fprintln(os.Stderr, buf)

		buf = buf[m[1]:]
		if !pressedS {
			pressedS = true
			fmt.Fprint(ser, "s")
			time.Sleep(250 * time.Millisecond)
			return true, buf, "Pressed S"
		} else {
			return true, buf, ""
		}
	} else if m := jumpersMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		return true, buf, ""
	} else if m := becauseJumperMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		return true, buf, ""
	} else if m := batteryMsg.FindStringIndex(buf); m != nil {
		fmt.Fprintln(os.Stderr, buf)
		buf = buf[m[1]:]
		pressedS = false
		reconfigure = false                                  // Starting a good run
		return true, serialWriteln(ser, buf, "4"), "Battery" // LiFePo4
	} else if m := batteryAltMsg.FindStringIndex(buf); m != nil {
		fmt.Fprintln(os.Stderr, buf)
		buf = buf[m[1]:]
		pressedS = false
		reconfigure = false                                  // Starting a good run
		return true, serialWriteln(ser, buf, "4"), "Battery" // LiFePo4
	} else if m := resetToDefaultMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		return true, serialWriteln(ser, buf, "n"), "no reset to default"
	} else if m := maxVoltsMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		return true, serialWriteln(ser, buf, "14.5"), "Max charge voltage"
	} else if m := maxAmpsMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		return true, serialWriteln(ser, buf, "10.0"), "Max charge current"
	} else if m := minAmpsMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		return true, serialWriteln(ser, buf, "0.25"), "Min charge current"
	} else if m := rechargeMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		return true, serialWriteln(ser, buf, "13.68"), "Recharge voltage"
	} else if m := maxMinMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		return true, serialWriteln(ser, buf, "1500"), "Charge minutes"
	} else if m := retryAfterMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		return true, serialWriteln(ser, buf, "30"), "Retry minutes"
	} else if m := minSupplyMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		return true, serialWriteln(ser, buf, "13"), "Min supply voltage"
	} else if m := trickleAmpsMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		reconfigure = true
		return true, serialWriteln(ser, buf, "0"), "Trickle current (will reconfigure)"
	} else if m := trickleVoltsMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		reconfigure = true
		return true, serialWriteln(ser, buf, "14"), "Trickle voltage (will reconfigure)"
	} else if m := minTempMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		return true, serialWriteln(ser, buf, "35"), "Min charge temp"
	} else if m := maxTempMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		return true, serialWriteln(ser, buf, "110"), "Max charge temp"
	} else if m := tempAdjustMsg.FindStringIndex(buf); m != nil {
		buf = buf[m[1]:]
		pressedS = false
		return true, serialWriteln(ser, buf, "n"), "Temperature voltage adjust"
	}
	return false, buf, ""
}

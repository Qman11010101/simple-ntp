package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

const WINDOW_TITLE = "simple-ntp"

const NTP_PORT = 123
const NTP_TIMEOUT = 10

const UNIX_TIME_OFFSET = 2208988800

type MyMainWindow struct {
	*walk.MainWindow
	hostUrl    *walk.LineEdit
	portNumber *walk.NumberEdit
	timeout    *walk.NumberEdit

	ipv4Display *walk.CheckBox
	msDisplay   *walk.CheckBox

	leapIndicator      *walk.LineEdit
	versionNumber      *walk.LineEdit
	mode               *walk.LineEdit
	stratum            *walk.LineEdit
	pollInterval       *walk.LineEdit
	precision          *walk.LineEdit
	rootDelay          *walk.LineEdit
	rootDispersion     *walk.LineEdit
	referenceID        *walk.LineEdit
	referenceTimestamp *walk.LineEdit
	originTimestamp    *walk.LineEdit
	receiveTimestamp   *walk.LineEdit
	transmitTimestamp  *walk.LineEdit
}

func reqNtp(host string, port int, timeout int) ([]byte, error) {
	ntpQuery := make([]byte, 48)
	ntpQuery[0] = 0x1b
	s, err := net.Dial("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return ntpQuery, err
	}
	defer s.Close()

	s.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Second))

	_, err = s.Write(ntpQuery)
	if err != nil {
		return ntpQuery, err
	}

	_, err = s.Read(ntpQuery)
	if err != nil {
		return ntpQuery, err
	}

	return ntpQuery, err
}

func desc(number int, describe string) string {
	return strconv.Itoa(number) + " (" + describe + ")"
}

func parseNtpShortFormat(t []byte) float64 {
	s := float64(binary.BigEndian.Uint16(t[0:2]))
	f := float64(binary.BigEndian.Uint16(t[2:4])) * math.Pow(2, -16)
	return s + f
}

func parseNtpTimestampFormat(t []byte) float64 {
	s := float64(binary.BigEndian.Uint32(t[0:4]))
	f := float64(binary.BigEndian.Uint32(t[4:8])) * math.Pow(2, -32)
	return s + f
}

func parseNtpTimestampFormatSplit(t []byte) (int, int) {
	seconds := int(binary.BigEndian.Uint32(t[0:4]))
	fractionNano := int(float64(binary.BigEndian.Uint32(t[4:8])) * math.Pow(2, -32) * 1000000000)
	return seconds, fractionNano
}

func convertNtpTimestampToIso8601ExtendedFormat(s int, f int) string {
	s -= UNIX_TIME_OFFSET
	t := time.Unix(int64(s), int64(f))
	return t.Format(time.RFC3339Nano)
}

func parseNtpBytes(ntpQuery []byte, ipv4Disp bool, msDisp bool) ([]string, error) {
	// NTP Query is 48 Bytes
	// Leap Indicator、Version Number、Modeを1byte目から取る
	firstByte := ntpQuery[0]
	leapIndicatorByte := int(firstByte >> 6)
	versionNumber := int(firstByte >> 3)
	modeByte := int(firstByte & 0x07)

	var leapIndicator string
	if leapIndicatorByte == 0 {
		leapIndicator = desc(leapIndicatorByte, "No Warning")
	} else if leapIndicatorByte == 1 {
		leapIndicator = desc(leapIndicatorByte, "Last minute of the day has 61 seconds")
	} else if leapIndicatorByte == 2 {
		leapIndicator = desc(leapIndicatorByte, "Last minute of the day has 59 seconds")
	} else {
		leapIndicator = desc(leapIndicatorByte, "Unknown")
	}

	var mode string
	if modeByte == 0 {
		mode = desc(modeByte, "Reserved")
	} else if modeByte == 1 {
		mode = desc(modeByte, "Symmetric Active")
	} else if modeByte == 2 {
		mode = desc(modeByte, "Symmetric Passive")
	} else if modeByte == 3 {
		mode = desc(modeByte, "Client")
	} else if modeByte == 4 {
		mode = desc(modeByte, "Server")
	} else if modeByte == 5 {
		mode = desc(modeByte, "Broadcast")
	} else if modeByte == 6 {
		mode = desc(modeByte, "NTP Control Message")
	} else if modeByte == 7 {
		mode = desc(modeByte, "Reserved for private use")
	} else {
		mode = desc(modeByte, "Unknown")
	}

	// Stratum
	stratumByte := int(ntpQuery[1])

	var stratum string
	if stratumByte == 0 {
		stratum = desc(stratumByte, "Unspecified or invalid")
	} else if stratumByte == 1 {
		stratum = desc(stratumByte, "Primary Server")
	} else if stratumByte >= 2 && stratumByte <= 15 {
		stratum = desc(stratumByte, "Secondary Server")
	} else if stratumByte == 16 {
		stratum = desc(stratumByte, "Unsynchronized")
	} else {
		stratum = desc(stratumByte, "Reserved")
	}

	// Poll
	pollIntervalByte := int(ntpQuery[2])
	pollInterval := strconv.Itoa(int(math.Pow(2, float64(pollIntervalByte)))) + " seconds"
	fmt.Printf("Poll interval: %s\n", pollInterval)

	var timeUnit string
	if msDisp {
		timeUnit = " ms"
	} else {
		timeUnit = " seconds"
	}

	precisionByte := int(int8(ntpQuery[3]))
	precisionSeconds := math.Pow(2, float64(precisionByte))
	if msDisp {
		precisionSeconds *= 1000
	}
	precision := strconv.FormatFloat((precisionSeconds), 'f', -1, 64) + timeUnit
	fmt.Printf("Precision: %s\n", precision)

	// Root Delay
	rootDelayBytes := ntpQuery[4:8]
	rootDelaySeconds := parseNtpShortFormat(rootDelayBytes)
	if msDisp {
		rootDelaySeconds *= 1000
	}
	rootDelay := strconv.FormatFloat(rootDelaySeconds, 'f', -1, 64) + timeUnit
	fmt.Printf("Root delay: %s\n", rootDelay)

	// Root Dispersion
	rootDispersionBytes := ntpQuery[8:12]
	rootDispersionSeconds := parseNtpShortFormat(rootDispersionBytes)
	if msDisp {
		rootDispersionSeconds *= 1000
	}
	rootDispersion := strconv.FormatFloat(rootDispersionSeconds, 'f', -1, 64) + timeUnit
	fmt.Printf("Root dispersion: %s\n", rootDispersion)

	// Reference ID
	referenceIDByte := ntpQuery[12:16]
	var referenceID string
	if stratumByte == 1 {
		for _, b := range referenceIDByte {
			if int(b) != 0 {
				referenceID += string(b)
			}
		}

	} else {
		for _, b := range referenceIDByte {
			referenceID += fmt.Sprintf("%02X", b)
		}
		// TODO: IPアドレス変換
		var referenceIDArrString []string
		for _, b := range referenceIDByte {
			referenceIDArrString = append(referenceIDArrString, strconv.Itoa(int(b)))
		}
		IPFormReferenceID := strings.Join(referenceIDArrString, ".")
		if ipv4Disp {
			referenceID += " (IPv4 form: " + IPFormReferenceID + ")"
		}
	}
	fmt.Printf("Reference ID: %s\n", referenceID)

	// Reference Timestamp
	referenceTimestampByte := ntpQuery[16:24]
	referenceTimestampSecond, referenceTimestampFraction := parseNtpTimestampFormatSplit(referenceTimestampByte)
	referenceTimestamp := strconv.FormatFloat(parseNtpTimestampFormat(referenceTimestampByte), 'f', -1, 64) + " (" +
		convertNtpTimestampToIso8601ExtendedFormat(referenceTimestampSecond, referenceTimestampFraction) + ")"

	// Origin Timestamp
	originTimestampByte := ntpQuery[24:32]
	originTimestampSecond, originTimestampFraction := parseNtpTimestampFormatSplit(originTimestampByte)
	originTimestamp := strconv.FormatFloat(parseNtpTimestampFormat(originTimestampByte), 'f', -1, 64) + " (" +
		convertNtpTimestampToIso8601ExtendedFormat(originTimestampSecond, originTimestampFraction) + ")"

	// Receive Timestamp
	receiveTimestampByte := ntpQuery[32:40]
	receiveTimestampSecond, receiveTimestampFraction := parseNtpTimestampFormatSplit(receiveTimestampByte)
	recieveTimestamp := strconv.FormatFloat(parseNtpTimestampFormat(receiveTimestampByte), 'f', -1, 64) + " (" +
		convertNtpTimestampToIso8601ExtendedFormat(receiveTimestampSecond, receiveTimestampFraction) + ")"

	// Transmit Timestamp
	transmitTimestampByte := ntpQuery[40:48]
	transmitTimestampSecond, transmitTimestampFraction := parseNtpTimestampFormatSplit(transmitTimestampByte)
	transmitTimestamp := strconv.FormatFloat(parseNtpTimestampFormat(transmitTimestampByte), 'f', -1, 64) + " (" +
		convertNtpTimestampToIso8601ExtendedFormat(transmitTimestampSecond, transmitTimestampFraction) + ")"

	return []string{
		leapIndicator,
		strconv.Itoa(versionNumber),
		mode,
		stratum,
		pollInterval,
		precision,
		rootDelay,
		rootDispersion,
		referenceID,
		referenceTimestamp,
		originTimestamp,
		recieveTimestamp,
		transmitTimestamp,
	}, nil
}

func main() {
	mw := &MyMainWindow{}

	icon, err := walk.Resources.Icon("2")
	if err != nil {
		log.Fatal(err)
	}

	mainWindowObject := MainWindow{
		AssignTo: &mw.MainWindow,
		Title:    WINDOW_TITLE,
		Icon:     icon,
		Size:     Size{Width: 400, Height: 480},
		MinSize:  Size{Width: 300, Height: 200},
		Layout:   VBox{},
		Children: []Widget{
			TextLabel{
				Text: "Input NTP server host (e.g. ntp.nict.jp, time.cloudflare.com, time.google.com, time.windows.com, etc...)",
			},
			TextLabel{
				Text: "Do not execute many times in a short time!",
			},
			Composite{
				Layout: Grid{Columns: 3},
				Children: []Widget{
					Label{
						Text: "NTP server host:",
					},
					LineEdit{
						AssignTo:   &mw.hostUrl,
						ColumnSpan: 2,
					},
					Label{
						Text: "Port number (Optional):",
					},
					NumberEdit{
						AssignTo: &mw.portNumber,
						Value:    float64(NTP_PORT),
					},
					HSpacer{},
					Label{
						Text: "Timeout seconds (Optional):",
					},
					NumberEdit{
						AssignTo: &mw.timeout,
						Value:    float64(NTP_TIMEOUT),
					},
					HSpacer{},
					CheckBox{
						AssignTo:    &mw.ipv4Display,
						Text:        "Display IPv4 form of reference ID (e.g. 1942E605 -> 25.66.230.5)",
						ToolTipText: "Note that reference ID is NOT always an IPv4 address! More details: See RFC 5905 7.3.",
						ColumnSpan:  3,
					},
					CheckBox{
						AssignTo:    &mw.msDisplay,
						Text:        "Display time in milliseconds",
						ToolTipText: "Precision, Root delay, and Root dispersion are displayed in milliseconds. They do not apply to the Poll interval.",
						ColumnSpan:  3,
					},
					PushButton{
						Text: "Execute",
						OnClicked: func() {
							host := strings.TrimSpace(mw.hostUrl.Text())
							port := int(mw.portNumber.Value())
							timeout := int(mw.timeout.Value())
							if host == "" {
								walk.MsgBox(mw, "Error", "NTP server host is empty!", walk.MsgBoxIconError)
								return
							}
							if port < 0 || port > 65535 {
								port = NTP_PORT
							}
							if timeout < 0 {
								timeout = NTP_TIMEOUT
							}
							ntpQuery, err := reqNtp(host, port, timeout)
							if err != nil {
								walk.MsgBox(mw, "Error", "An error occurred during the request:\n"+err.Error(), walk.MsgBoxIconError)
								return
							}
							parsedNtpQuery, err := parseNtpBytes(ntpQuery, mw.ipv4Display.Checked(), mw.msDisplay.Checked())
							if err != nil {
								walk.MsgBox(mw, "Error", "An error occurred during the parsing:\n"+err.Error(), walk.MsgBoxIconError)
								return
							}
							mw.leapIndicator.SetText(parsedNtpQuery[0])
							mw.versionNumber.SetText(parsedNtpQuery[1])
							mw.mode.SetText(parsedNtpQuery[2])
							mw.stratum.SetText(parsedNtpQuery[3])
							mw.pollInterval.SetText(parsedNtpQuery[4])
							mw.precision.SetText(parsedNtpQuery[5])
							mw.rootDelay.SetText(parsedNtpQuery[6])
							mw.rootDispersion.SetText(parsedNtpQuery[7])
							mw.referenceID.SetText(parsedNtpQuery[8])
							mw.referenceTimestamp.SetText(parsedNtpQuery[9])
							mw.originTimestamp.SetText(parsedNtpQuery[10])
							mw.receiveTimestamp.SetText(parsedNtpQuery[11])
							mw.transmitTimestamp.SetText(parsedNtpQuery[12])
						},
					},
					HSpacer{
						ColumnSpan: 2,
					},
				},
			},
			Composite{
				Layout: Grid{Columns: 2},
				Children: []Widget{
					Label{
						Text: "Leap Indicator:",
					},
					LineEdit{
						AssignTo: &mw.leapIndicator,
						ReadOnly: true,
					},
					Label{
						Text: "Version Number:",
					},
					LineEdit{
						AssignTo: &mw.versionNumber,
						ReadOnly: true,
					},
					Label{
						Text: "Mode:",
					},
					LineEdit{
						AssignTo: &mw.mode,
						ReadOnly: true,
					},
					Label{
						Text: "Stratum:",
					},
					LineEdit{
						AssignTo: &mw.stratum,
						ReadOnly: true,
					},
					Label{
						Text: "Poll Interval:",
					},
					LineEdit{
						AssignTo: &mw.pollInterval,
						ReadOnly: true,
					},
					Label{
						Text: "Precision:",
					},
					LineEdit{
						AssignTo: &mw.precision,
						ReadOnly: true,
					},
					Label{
						Text: "Root Delay:",
					},
					LineEdit{
						AssignTo: &mw.rootDelay,
						ReadOnly: true,
					},
					Label{
						Text: "Root Dispersion:",
					},
					LineEdit{
						AssignTo: &mw.rootDispersion,
						ReadOnly: true,
					},
					Label{
						Text: "Reference ID:",
					},
					LineEdit{
						AssignTo: &mw.referenceID,
						ReadOnly: true,
					},
					Label{
						Text: "Reference Timestamp:",
					},
					LineEdit{
						AssignTo: &mw.referenceTimestamp,
						ReadOnly: true,
					},
					Label{
						Text: "Origin Timestamp:",
					},
					LineEdit{
						AssignTo:    &mw.originTimestamp,
						ReadOnly:    true,
						ToolTipText: "This value is normally 0 when you are using this tool.",
					},
					Label{
						Text: "Receive Timestamp:",
					},
					LineEdit{
						AssignTo: &mw.receiveTimestamp,
						ReadOnly: true,
					},
					Label{
						Text: "Transmit Timestamp:",
					},
					LineEdit{
						AssignTo: &mw.transmitTimestamp,
						ReadOnly: true,
					},
				},
			},
		},
	}

	if _, err := mainWindowObject.Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

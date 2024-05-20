package awt

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/webtransport-go"
)

// relevant QLOG entry
type QLogEntry struct {
	// offset time from reference time in ms
	Time float64 `json:"time"`
	// name of the event
	Name string `json:"name"`
	// relevant data
	Data struct {
		SmoothedRTT      float64 `json:"smoothed_rtt"`
		LatestRTT        float64 `json:"latest_rtt"`
		RTTVariance      float64 `json:"rtt_variance"`
		CongestionWindow uint64  `json:"congestion_window,omitempty"`
		BytesInFlight    uint64  `json:"bytes_in_flight"`
		PacketsInFlight  uint64  `json:"packets_in_flight,omitempty"`
	} `json:"data"`
}

func GetQlogPath(session *webtransport.Session, allQlogPaths *[]string) (p string, err error) {
	var (
		id quic.ConnectionTracingID
		ok bool
	)

	if id, ok = session.Context().Value(quic.ConnectionTracingKey).(quic.ConnectionTracingID); !ok {
		err = ErrInvalidConnectionTracingKey
		return
	}

	if int(id) > len(*allQlogPaths) {
		err = ErrInvalidConnectionTracingKey
		return
	}

	p = (*allQlogPaths)[id-1]

	return
}

type Qlog struct {
	// path to qlog file
	Path string

	// reference time from qlog file
	refTime time.Time
}

func NewQlog(path string) (q Qlog, err error) {
	q = Qlog{
		Path: path,
	}

	if err = q.setRefTime(); err != nil {
		return
	}

	return
}

func (q *Qlog) GetTimestampETP(from, to time.Duration) (etp uint64, err error) {
	var (
		buf       []byte
		lines     [][]byte
		etpValues []uint64
	)

	if buf, err = os.ReadFile(q.Path); err != nil {
		return
	}

	lines = bytes.Split(buf, []byte("\n"))

	for _, line := range lines {
		var (
			entry QLogEntry
			value uint64
		)

		if err = json.Unmarshal(line, &entry); err != nil {
			return
		}

		if entry.Time < float64(from) {
			continue
		}
		if entry.Time > float64(to) {
			break
		}

		value = uint64((float64(entry.Data.BytesInFlight) * 8) / entry.Data.SmoothedRTT)
		etpValues = append(etpValues, value)
	}

	if len(etpValues) <= 0 {
		return
	}

	etp = sum(etpValues) / uint64(len(etpValues))

	return
}

func (q *Qlog) GetTimeSinceRefTime() time.Duration {
	return time.Since(q.refTime)
}

func (q *Qlog) setRefTime() (err error) {
	var (
		buf     []byte
		refStr  string
		regex   *regexp.Regexp = regexp.MustCompile(`"reference_time":(?P<time>\d*\.\d*)`)
		matches []string
		ref     float64
	)

	if buf, err = os.ReadFile(q.Path); err != nil {
		return
	}

	matches = regex.FindStringSubmatch(string(buf))

	refStr = matches[regex.SubexpIndex("time")]

	if ref, err = strconv.ParseFloat(refStr, 64); err != nil {
		return
	}

	q.refTime = time.Unix(0, int64(ref*float64(time.Millisecond)))

	return
}

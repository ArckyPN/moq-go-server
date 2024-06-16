package awt

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/webtransport-go"
)

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

type QLogEntry struct {
	Time *float64 `json:"time"`
	Name *string  `json:"name"`
	Data *struct {
		Header *struct {
			PacketType   *string `json:"packet_type"`
			PacketNumber *uint64 `json:"packet_number"`
			KeyPhaseBit  *string `json:"key_phase_bit"`
		} `json:"header"`
		Raw *struct {
			Length *uint64 `json:"length"`
		} `json:"raw"`
		Frames *[]struct {
			FrameType *string `json:"frame_type"`
			StreamID  *int64  `json:"stream_id"`
			Offset    *uint64 `json:"offset"`
			Length    *uint64 `json:"length"`
		} `json:"frames"`
	} `json:"data"`
}

func (q *QLogEntry) isNil() bool {
	if q.Time == nil {
		return true
	}
	if q.Data == nil {
		return true
	}
	if q.Data.Raw == nil {
		return true
	}
	if q.Data.Raw.Length == nil {
		return true
	}
	if q.Data.Frames == nil {
		return true
	}
	for _, frame := range *q.Data.Frames {
		if frame.StreamID == nil {
			return true
		}
	}

	return false
}

type Qlog struct {
	// path to qlog file
	path string
}

func NewQlog(path string) (q Qlog, err error) {
	q = Qlog{
		path: path,
	}

	return
}

func (q *Qlog) FromStreamID(streamID uint64, size int) (etp uint64, err error) {
	var (
		file       *os.File
		scanner    *bufio.Scanner
		regex      *regexp.Regexp
		buf        []byte
		start, end *float64
	)

	regex = regexp.MustCompile(fmt.Sprintf(`"stream_id":%d`, streamID))

	if file, err = os.Open(q.path); err != nil {
		log.Printf("Error: %s\n", err)
		return
	}
	defer file.Close()

	scanner = bufio.NewScanner(file)

	for scanner.Scan() {
		buf = scanner.Bytes()

		if !regex.Match(buf) {
			continue
		}
		var (
			entry QLogEntry
		)

		if err = json.Unmarshal(buf, &entry); err != nil {
			err = nil
			continue
		}
		if entry.isNil() {
			continue
		}

		if start == nil {
			start = entry.Time
			continue
		}
		end = entry.Time
	}

	if start == nil || end == nil {
		return
	}

	etp = uint64(float64(size*8) / (*end - *start))

	return
}

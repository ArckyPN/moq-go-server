package awt

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
)

const (
	READ_BLOCK_SIZE  = 1024
	MAX_VAR_INT_SIZE = 64
)

type LocPackager struct {
	MediaType       string
	Timestamp       uint64
	Duration        uint64
	ChunkType       string
	SeqId           uint64
	FirstFrameClkms uint64
	PId             string
	MetaData        []byte
	Data            []byte
}

func NewLocPackager() LocPackager {
	return LocPackager{
		MediaType:       "",
		Timestamp:       0,
		Duration:        0,
		ChunkType:       "",
		SeqId:           0,
		FirstFrameClkms: 0,
		PId:             "",
		MetaData:        []byte{},
		Data:            []byte{},
	}
}

func (loc *LocPackager) ToString() (str string) {
	str = "LocPackage:\n"
	str += fmt.Sprintln("\tMediaType: ", loc.MediaType)
	str += fmt.Sprintln("\tTimestamp: ", loc.Timestamp)
	str += fmt.Sprintln("\tDuration: ", loc.Duration)
	str += fmt.Sprintln("\tSeqId: ", loc.SeqId)
	str += fmt.Sprintln("\tFirstFrameClkms: ", loc.FirstFrameClkms)
	str += fmt.Sprintln("\tPId: ", loc.PId)
	str += fmt.Sprintln("\tMetaData: ", len(loc.MetaData))
	str += fmt.Sprintln("\tData: ", len(loc.Data))

	return
}

func (loc *LocPackager) Copy() (newLoc LocPackager) {
	newLoc.MediaType = loc.MediaType
	newLoc.Timestamp = loc.Timestamp
	newLoc.Duration = loc.Duration
	newLoc.ChunkType = loc.ChunkType
	newLoc.SeqId = loc.SeqId
	newLoc.FirstFrameClkms = loc.FirstFrameClkms
	newLoc.PId = loc.PId
	copy(newLoc.MetaData, loc.MetaData)
	copy(newLoc.Data, loc.Data)
	return
}

func (loc *LocPackager) SetData(MediaType string, Timestamp uint64, Duration uint64, ChunkType string, SeqId uint64, FirstFrameClkms uint64, MetaData []byte, Data []byte) {
	var (
		PId string = base64.RawStdEncoding.EncodeToString([]byte(fmt.Sprintf("%s-%d-%s-%d-%d", MediaType, Timestamp, ChunkType, SeqId, rand.Int64N(100_000))))
	)
	loc.MediaType = MediaType
	loc.Timestamp = Timestamp
	loc.Duration = Duration
	loc.ChunkType = ChunkType
	loc.SeqId = SeqId
	loc.FirstFrameClkms = FirstFrameClkms
	loc.PId = PId
	loc.MetaData = MetaData
	loc.Data = Data
}

func (loc *LocPackager) Decode(reader io.Reader) (err error) {
	var (
		mediaTypeInt, chunkTypeInt, metaDataSize uint64

		metaData []byte
	)
	if mediaTypeInt, err = varIntToNumber(reader); err != nil {
		return
	}
	switch mediaTypeInt {
	case 0:
		loc.MediaType = "data"
	case 1:
		loc.MediaType = "audio"
	case 2:
		loc.MediaType = "video"
	default:
		return fmt.Errorf("invalid mediaType")
	}

	if chunkTypeInt, err = varIntToNumber(reader); err != nil {
		return
	}
	switch chunkTypeInt {
	case 0:
		loc.ChunkType = "delta"
	case 1:
		loc.ChunkType = "key"
	default:
		return fmt.Errorf("invalid chunkType")
	}

	if loc.SeqId, err = varIntToNumber(reader); err != nil {
		return
	}
	if loc.Timestamp, err = varIntToNumber(reader); err != nil {
		return
	}
	if loc.Duration, err = varIntToNumber(reader); err != nil {
		return
	}
	if loc.FirstFrameClkms, err = varIntToNumber(reader); err != nil {
		return
	}

	if metaDataSize, err = varIntToNumber(reader); err != nil {
		return
	}
	if metaDataSize > 0 {
		metaData = make([]byte, metaDataSize)
		if err = readStream(reader, 0, int(metaDataSize), &metaData); err != nil {
			return
		}
		loc.MetaData = metaData
	}

	if loc.Data, err = io.ReadAll(reader); err != nil {
		return
	}

	return
}

func (loc *LocPackager) Encode() (buf []byte, err error) {
	var (
		mediaTypeBytes, chunkTypeBytes, seqIdBytes, timestampBytes, durationBytes, firstFrameClkmsBytes, metaDataSizeBytes []byte

		metaDataSize int = len(loc.MetaData)
	)
	switch loc.MediaType {
	case "data":
		if mediaTypeBytes, err = numberToVarInt(0); err != nil {
			return
		}
	case "audio":
		if mediaTypeBytes, err = numberToVarInt(1); err != nil {
			return
		}
	case "video":
		if mediaTypeBytes, err = numberToVarInt(2); err != nil {
			return
		}
	default:
		return nil, errors.New("invalid mediaType")
	}

	switch loc.ChunkType {
	case "delta":
		if chunkTypeBytes, err = numberToVarInt(0); err != nil {
			return
		}
	case "key":
		if chunkTypeBytes, err = numberToVarInt(1); err != nil {
			return
		}
	default:
		return nil, errors.New("invalid chunkType")
	}

	if seqIdBytes, err = numberToVarInt(loc.SeqId); err != nil {
		return
	}
	if timestampBytes, err = numberToVarInt(loc.Timestamp); err != nil {
		return
	}
	if durationBytes, err = numberToVarInt(loc.Duration); err != nil {
		return
	}
	if firstFrameClkmsBytes, err = numberToVarInt(loc.FirstFrameClkms); err != nil {
		return
	}

	if metaDataSizeBytes, err = numberToVarInt(uint64(metaDataSize)); err != nil {
		return
	}

	buf = append(buf, mediaTypeBytes...)
	buf = append(buf, chunkTypeBytes...)
	buf = append(buf, seqIdBytes...)
	buf = append(buf, timestampBytes...)
	buf = append(buf, durationBytes...)
	buf = append(buf, firstFrameClkmsBytes...)
	buf = append(buf, metaDataSizeBytes...)
	buf = append(buf, loc.MetaData...)
	buf = append(buf, loc.Data...)

	return
}

const (
	MAX_U6  = 63
	MAX_U14 = 16383
	MAX_U30 = 1073741823
	MAX_U53 = 9007199254740990
)

func varIntToNumber(reader io.Reader) (num uint64, err error) {
	var (
		buf  []byte = make([]byte, 1, MAX_VAR_INT_SIZE)
		size uint8
	)

	if err = readStream(reader, 0, 1, &buf); err != nil {
		return
	}

	size = (uint8(buf[0]) & 0xc0) >> 6
	switch size {
	case 0:
		num = uint64(uint8(buf[0]) & 0x3f)
	case 1:
		if err = readStream(reader, 1, 1, &buf); err != nil {
			return
		}
		num = uint64(binary.BigEndian.Uint16(buf) & 0x3fff)
	case 2:
		if err = readStream(reader, 1, 3, &buf); err != nil {
			return
		}
		num = uint64(binary.BigEndian.Uint32(buf) & 0x3fffffff)
	case 3:
		if err = readStream(reader, 1, 7, &buf); err != nil {
			return
		}
		num = uint64(binary.BigEndian.Uint64(buf) & 0x3fffffffffffffff)
	default:
		err = errors.New("impossible")
	}

	return
}

func numberToVarInt(num uint64) (buf []byte, err error) {
	switch {
	case num <= MAX_U6:
		buf = make([]byte, 1)
		buf[0] = byte(num)
	case num <= MAX_U14:
		buf = make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(num|0x4000))
	case num <= MAX_U30:
		buf = make([]byte, 4)
		binary.BigEndian.PutUint32(buf, uint32(num|0x80000000))
	case num <= MAX_U53:
		buf = make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(num|0xc000000000000000))
	default:
		return nil, errors.New("overflow, num larger than 53 bit")
	}
	return
}

func readStream(reader io.Reader, offset, size int, buf *[]byte) (err error) {
	// create new buffer of size to read next chunk
	var extend []byte = make([]byte, size)

	// read chunk
	if _, err = reader.Read(extend); err != nil {
		return
	}

	// extend capacity of buf to accommodate new chunk
	*buf = (*buf)[:size+offset]

	// join buf and extension
	for i := offset; i < offset+size; i++ {
		(*buf)[i] = extend[i-offset]
	}

	return
}

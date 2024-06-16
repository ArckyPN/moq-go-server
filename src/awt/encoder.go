package awt

type EncoderQuality struct {
	Bitrate    uint64
	Resolution string
	MaxRate    uint64
	BufSize    uint64
}

var EncoderSettings []EncoderQuality = []EncoderQuality{
	{
		Bitrate:    100,
		Resolution: "256x144",
		MaxRate:    100,
		BufSize:    150,
	},
	{
		Bitrate:    500,
		Resolution: "640x360",
		MaxRate:    500,
		BufSize:    750,
	},
	{
		Bitrate:    1_000,
		Resolution: "1920x1080",
		MaxRate:    1_000,
		BufSize:    1_500,
	},
}

func Encode(chunk []byte, quality EncoderQuality) (eChunk []byte, err error) {
	// var (
	// 	input          *bytes.Buffer = bytes.NewBuffer(chunk)
	// 	output, errOut bytes.Buffer
	// 	// TODO if laptops allow it, maybe inscribe video with resolution and bitrate
	// 	command *exec.Cmd = exec.Command(
	// 		"ffmpeg",
	// 		"-i", "pipe:0",
	// 		"-preset", "ultrafast",
	// 		"-c", "copy",
	// 		"-f", "matroska",
	// 		"-vf", quality.Resolution,
	// 		"-b:v", fmt.Sprintf("%dk", quality.Bitrate),
	// 		"-an",
	// 		"-maxrate", fmt.Sprintf("%dk", quality.MaxRate),
	// 		"-bufsize", fmt.Sprintf("%dk", quality.BufSize),
	// 		"pipe:1",
	// 	)
	// )

	// command.Stdin = input
	// command.Stdout = &output
	// command.Stderr = &errOut

	// if err = command.Run(); err != nil {
	// 	return
	// }

	// // TODO check input len and output len

	// fmt.Printf("Err: %s\n", errOut.String())
	// eChunk = output.Bytes()
	eChunk = make([]byte, len(chunk))
	copy(eChunk, chunk)

	return
}

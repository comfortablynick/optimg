package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/discordapp/lilliput"
)

const debug_mode bool = false

type Options struct {
	version        bool
	debug          bool
	inputFilename  string
	outputFilename string
	outputWidth    int
	outputHeight   int
	maxWidth       int
	maxHeight      int
	maxLongest     int
	pctResize      int
	stretch        bool
	force          bool
	additionalArgs []string
}

var EncodeOptions = map[string]map[int]int{
	".jpeg": map[int]int{lilliput.JpegQuality: 85},
	".png":  map[int]int{lilliput.PngCompression: 7},
	".webp": map[int]int{lilliput.WebpQuality: 85},
}

var opt Options

// Max calculates the maximum of two integers
func Max(nums ...int) int {
	max := nums[0]
	for _, i := range nums[1:] {
		if i > max {
			max = i
		}
	}
	return max
}

// Scale calculates the new pixel size based on pct scaling factor
func Scale(pct int, size int) int {
	return int(float64(size) * (float64(pct) / float64(100)))
}

// Check if output file is valid
func validateOutputFile(filename string, force bool) error {
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		if !force {
			return fmt.Errorf("error: output filename %s exists. To overwrite, use -f to force.", filename)
		}
		if err := os.Remove(filename); err != nil {
			return fmt.Errorf("error: unable to remove existing file %s; aborting.", filename)
		}
		fmt.Println("output file exists; replacing due to -f.")
	}
	return nil
}

func init() {
	// init log
	// TODO: add time, etc.
	log.SetPrefix("DEBUG ")
	log.SetFlags(log.Lshortfile)
	log.SetOutput(ioutil.Discard)

	// set flags
	flag.StringVar(&opt.inputFilename, "i", "", "name of input file to resize/transcode")
	flag.StringVar(&opt.outputFilename, "o", "", "name of output file, also determines output type")
	flag.IntVar(&opt.outputWidth, "w", 0, "width of output file")
	flag.IntVar(&opt.outputHeight, "h", 0, "height of output file")
	flag.IntVar(&opt.maxWidth, "mw", 0, "maximum width of output file")
	flag.IntVar(&opt.maxHeight, "mh", 0, "maximum height of output file")
	flag.IntVar(&opt.maxLongest, "m", 0, "maximum length of either dimension")
	flag.IntVar(&opt.pctResize, "pct", 0, "resize to pct of original dimensions")
	flag.BoolVar(&opt.stretch, "stretch", false, "perform stretching resize instead of cropping")
	flag.BoolVar(&opt.force, "f", false, "overwrite output file if it exists")
	flag.BoolVar(&opt.debug, "d", false, "print debug messages to console")
	flag.Parse()
	opt.additionalArgs = flag.Args()

	if debug_mode || opt.debug {
		log.SetOutput(os.Stderr)
	}
}

func main() {
	log.Printf("Command line options: %+v", opt)
	if opt.inputFilename == "" {
		fmt.Println("No input filename provided, quitting.")
		flag.Usage()
		os.Exit(1)
	}

	// decoder wants []byte, so read the whole file into a buffer
	inputBuf, err := ioutil.ReadFile(opt.inputFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input file, %s\n", err)
		os.Exit(1)
	}

	// check if output file is valid
	if opt.outputFilename == "" {
		opt.outputFilename = "resized" + filepath.Ext(opt.inputFilename)
	}

	if err := validateOutputFile(opt.outputFilename, opt.force); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	decoder, err := lilliput.NewDecoder(inputBuf)
	// this error reflects very basic checks,
	// mostly just for the magic bytes of the file to match known image formats
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decoding image: %s\n", err)
		os.Exit(1)
	}
	defer decoder.Close()

	header, err := decoder.Header()
	// this error is much more comprehensive and reflects
	// format errors
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading image header: %s\n", err)
		os.Exit(1)
	}

	// print some basic info about the image
	fmt.Printf("file type: %s\n", decoder.Description())
	fmt.Printf("%dpx x %dpx\n", header.Width(), header.Height())

	if decoder.Duration() != 0 {
		fmt.Printf("duration: %.2f s\n", float64(decoder.Duration())/float64(time.Second))
	}

	// get ready to resize image,
	// using 8192x8192 maximum resize buffer size
	ops := lilliput.NewImageOps(8192)
	defer ops.Close()

	// create a buffer to store the output image, 50MB in this case
	outputImg := make([]byte, 50*1024*1024)

	// use user supplied filename to guess output type if provided
	// otherwise don't transcode (use existing type)
	outputType := "." + strings.ToLower(decoder.Description())
	if opt.outputFilename != "" {
		outputType = filepath.Ext(opt.outputFilename)
	}

	if opt.pctResize > 0 {
		opt.outputWidth = Scale(header.Width(), opt.pctResize)
		opt.outputHeight = Scale(header.Height(), opt.pctResize)
	}

	if opt.maxWidth > 0 {
		opt.outputWidth = opt.maxWidth
	}

	if opt.maxHeight > 0 {
		opt.outputHeight = opt.maxHeight
	}

	if opt.maxLongest > 0 {
		// opt.out
	}

	if opt.outputWidth == 0 {
		opt.outputWidth = header.Width()
	}

	if opt.outputHeight == 0 {
		opt.outputHeight = header.Height()
	}

	resizeMethod := lilliput.ImageOpsFit
	if opt.stretch {
		resizeMethod = lilliput.ImageOpsResize
	}

	if opt.outputWidth == header.Width() && opt.outputHeight == header.Height() {
		resizeMethod = lilliput.ImageOpsNoResize
	} else {
		fmt.Printf("image resized to %dpx x %dpx\n", opt.outputWidth, opt.outputHeight)
	}

	opts := &lilliput.ImageOptions{
		FileType:             outputType,
		Width:                opt.outputWidth,
		Height:               opt.outputHeight,
		ResizeMethod:         resizeMethod,
		NormalizeOrientation: true,
		EncodeOptions:        EncodeOptions[outputType],
	}

	// resize and transcode image
	outputImg, err = ops.Transform(decoder, opts, outputImg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error transforming image: %s\n", err)
		os.Exit(1)
	}

	err = ioutil.WriteFile(opt.outputFilename, outputImg, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing resized image: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("image written to %s\n", opt.outputFilename)
}

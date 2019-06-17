package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/discordapp/lilliput"
)

var EncodeOptions = map[string]map[int]int{
	".jpeg": map[int]int{lilliput.JpegQuality: 85},
	".png":  map[int]int{lilliput.PngCompression: 7},
	".webp": map[int]int{lilliput.WebpQuality: 85},
}

func main() {
	var inputFilename string
	var outputWidth int
	var outputHeight int
	var pctResize int
	var outputFilename string
	var stretch bool
	var force bool

	flag.StringVar(&inputFilename, "input", "", "name of input file to resize/transcode")
	flag.StringVar(&outputFilename, "output", "", "name of output file, also determines output type")
	flag.IntVar(&outputWidth, "width", 0, "width of output file")
	flag.IntVar(&outputHeight, "height", 0, "height of output file")
	flag.IntVar(&pctResize, "pct", 0, "resize to pct of original dimensions")
	flag.BoolVar(&stretch, "stretch", false, "perform stretching resize instead of cropping")
	flag.BoolVar(&force, "force", false, "overwrite output file if it exists")
	flag.Parse()

	if inputFilename == "" {
		fmt.Printf("No input filename provided, quitting.\n")
		flag.Usage()
		os.Exit(1)
	}

	// decoder wants []byte, so read the whole file into a buffer
	inputBuf, err := ioutil.ReadFile(inputFilename)
	if err != nil {
		fmt.Printf("failed to read input file, %s\n", err)
		os.Exit(1)
	}

	decoder, err := lilliput.NewDecoder(inputBuf)
	// this error reflects very basic checks,
	// mostly just for the magic bytes of the file to match known image formats
	if err != nil {
		fmt.Printf("error decoding image, %s\n", err)
		os.Exit(1)
	}
	defer decoder.Close()

	header, err := decoder.Header()
	// this error is much more comprehensive and reflects
	// format errors
	if err != nil {
		fmt.Printf("error reading image header, %s\n", err)
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
	if outputFilename != "" {
		outputType = filepath.Ext(outputFilename)
	}

	if pctResize != 0 {
		outputWidth = int(float64(header.Width()) * (float64(pctResize) / float64(100)))
		outputHeight = int(float64(header.Height()) * (float64(pctResize) / float64(100)))
	}

	if outputWidth == 0 {
		outputWidth = header.Width()
	}

	if outputHeight == 0 {
		outputHeight = header.Height()
	}

	resizeMethod := lilliput.ImageOpsFit
	if stretch {
		resizeMethod = lilliput.ImageOpsResize
	}

	if outputWidth == header.Width() && outputHeight == header.Height() {
		resizeMethod = lilliput.ImageOpsNoResize
	} else {
		fmt.Printf("image resized to %dpx x %dpx\n", outputWidth, outputHeight)
	}

	opts := &lilliput.ImageOptions{
		FileType:             outputType,
		Width:                outputWidth,
		Height:               outputHeight,
		ResizeMethod:         resizeMethod,
		NormalizeOrientation: true,
		EncodeOptions:        EncodeOptions[outputType],
	}

	// resize and transcode image
	outputImg, err = ops.Transform(decoder, opts, outputImg)
	if err != nil {
		fmt.Printf("error transforming image, %s\n", err)
		os.Exit(1)
	}

	// image has been resized, now write file out
	if outputFilename == "" {
		outputFilename = "resized" + filepath.Ext(inputFilename)
	}

	if _, err := os.Stat(outputFilename); !os.IsNotExist(err) {
		if !force {
			fmt.Printf("output filename %s exists, quitting\n", outputFilename)
			os.Exit(1)
		}
		if err := os.Remove(outputFilename); err != nil {
			fmt.Printf("error removing existing filename %s, quitting\n", outputFilename)
			os.Exit(1)
		}
		fmt.Println("output file exists; replacing due to -force")
	}

	err = ioutil.WriteFile(outputFilename, outputImg, 0644)
	if err != nil {
		fmt.Printf("error writing out resized image, %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("image written to %s\n", outputFilename)
}

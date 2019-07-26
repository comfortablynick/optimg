package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
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
	minShortest    int
	pctResize      float64
	stretch        bool
	force          bool
	noAction       bool
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

// Min calculates the minimum of two integers
func Min(nums ...int) int {
	min := nums[0]
	for _, i := range nums[1:] {
		if i < min {
			min = i
		}
	}
	return min
}

// Scale calculates the new pixel size based on pct scaling factor
func Scale(pct float64, size int) int {
	return int(float64(size) * (float64(pct) / float64(100)))
}

// Humanize prints bytes in human-readable strings
func Humanize(bytes int) string {
	suffix := "B"
	num := float64(bytes)
	factor := 1024.0
	// k=kilo, M=mega, G=giga, T=tera, P=peta, E=exa, Z=zetta, Y=yotta
	units := []string{"", "K", "M", "G", "T", "P", "E", "Z"}

	for _, unit := range units {
		if num < factor {
			return fmt.Sprintf("%3.1f%s%s", num, unit, suffix)
		}
		num = (num / factor)
	}
	// if we got here, it's a really big number!
	// return yottabytes
	return fmt.Sprintf("%.1f%s%s", num, "Y", suffix)
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
	flag.IntVar(&opt.maxLongest, "max", 0, "maximum length of either dimension")
	flag.IntVar(&opt.minShortest, "min", 0, "Minimum length of shortest side")
	flag.Float64Var(&opt.pctResize, "pct", 0, "resize to pct of original dimensions")
	flag.BoolVar(&opt.stretch, "stretch", false, "perform stretching resize instead of cropping")
	flag.BoolVar(&opt.force, "f", false, "overwrite output file if it exists")
	flag.BoolVar(&opt.debug, "d", false, "print debug messages to console")
	flag.BoolVar(&opt.noAction, "n", false, "don't write files; just display results")
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
		ext := filepath.Ext(opt.inputFilename)
		opt.outputFilename = strings.TrimSuffix(opt.inputFilename, ext) + "_opt" + ext
	}

	if !opt.noAction {
		if err := validateOutputFile(opt.outputFilename, opt.force); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		fmt.Println("**Displaying results only**")
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

	if opt.maxLongest > 0 {
		// calculate longest dim, and assign to pctResize
		if longest := Max(header.Width(), header.Height()); longest > opt.maxLongest {
			fmt.Printf("Resizing to longest dimension of %d px\n", opt.maxLongest)
			opt.pctResize = (float64(opt.maxLongest) / float64(longest)) * float64(100)
		}
	}

	if opt.minShortest > 0 {
		// calculate longest dim, and assign to pctResize
		if shortest := Min(header.Width(), header.Height()); shortest > opt.minShortest {
			fmt.Printf("Resizing shortest dimension to %d px\n", opt.minShortest)
			opt.pctResize = (float64(opt.minShortest) / float64(shortest)) * float64(100)
		}
	}

	opt.outputWidth = (func() int {
		if opt.pctResize > 0 {
			return Scale(opt.pctResize, header.Width())
		}
		if opt.maxWidth > 0 {
			return opt.maxWidth
		}
		if opt.outputWidth == 0 {
			return header.Width()
		}
		return opt.outputWidth
	})()

	opt.outputHeight = (func() int {
		if opt.pctResize > 0 {
			return Scale(opt.pctResize, header.Height())
		}
		if opt.maxHeight > 0 {
			return opt.maxHeight
		}
		if opt.outputHeight == 0 {
			return header.Height()
		}
		return opt.outputHeight
	})()

	resizeMethod := lilliput.ImageOpsFit
	if opt.stretch {
		resizeMethod = lilliput.ImageOpsResize
	}

	if opt.outputWidth == header.Width() && opt.outputHeight == header.Height() {
		resizeMethod = lilliput.ImageOpsNoResize
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

	if !opt.noAction {
		err = ioutil.WriteFile(opt.outputFilename, outputImg, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error writing resized image: %s\n", err)
			os.Exit(1)
		}
	}

	inputSize := len(inputBuf)
	outputSize := len(outputImg)
	log.Printf("Input buf size: %d", inputSize)
	log.Printf("Output buf size: %d", outputSize)

	// print some basic info about the image
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)

	fmt.Fprintf(w, "File Name\t%s\t -> \t%s\n", opt.inputFilename, opt.outputFilename)
	fmt.Fprintf(w, "File Dimensions\t%d x %d px\t -> \t%d x %d px\n",
		header.Width(), header.Height(), opt.outputWidth, opt.outputHeight)
	fmt.Fprintf(w, "File Size\t%s\t -> \t%s\n", Humanize(inputSize), Humanize(outputSize))
	fmt.Fprintf(w, "Size Reduction\t%.1f%%", 100.0-(float64(outputSize)/float64(inputSize)*100))

	w.Flush()     // write details table
	fmt.Println() // newline separator
}

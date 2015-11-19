package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"
	_ "golang.org/x/image/tiff"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
)

var (
	// When flagVerbose is true, logging output will be written to stderr.
	// Errors will always be written to stderr.
	flagVerbose bool

	// The initial width and height of the window.
	flagWidth, flagHeight int

	// If set, the image window will automatically resize to the first image
	// that it displays.
	flagAutoResize bool

	// The amount to increment panning when using h,j,k,l
	flagStepIncrement int

	// Whether to run a CPU profile.
	flagProfile string
)

func init() {
	// Set the prefix for verbose output.
	log.SetPrefix("[imgv] ")

	// Set all of the flags.
	flag.BoolVar(&flagVerbose, "v", false,
		"If set, logging output will be printed to stderr.")
	flag.IntVar(&flagWidth, "width", 600,
		"The initial width of the window.")
	flag.IntVar(&flagHeight, "height", 600,
		"The initial height of the window.")
	flag.BoolVar(&flagAutoResize, "auto-resize", false,
		"If set, window will resize to size of first image.")
	flag.IntVar(&flagStepIncrement, "increment", 20,
		"The increment (in pixels) used to pan the image.")
	flag.StringVar(&flagProfile, "profile", "",
		"If set, a CPU profile will be saved to the file name provided.")
	flag.Usage = usage
	flag.Parse()

	// Do some error checking on the flag values... naughty!
	if flagWidth == 0 || flagHeight == 0 {
		log.Fatal("The width and height must be non-zero values.")
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [flags] image-file [image-file ...]\n",
		basename(os.Args[0]))
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {
	// Run the CPU profile if we're instructed to.
	if len(flagProfile) > 0 {
		f, err := os.Create(flagProfile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// Whoops!
	if flag.NArg() == 0 {
		fmt.Fprint(os.Stderr, "\n")
		log.Print("No images specified.\n\n")
		usage()
	}

	driver.Main(func(s screen.Screen) {
		// Decode all images (in parallel).
		names, imgs := decodeImages(findFiles(flag.Args()))

		// Die now if we don't have any images!
		if len(imgs) == 0 {
			log.Fatal("No images specified could be shown. Quitting...")
		}

		winSize := image.Point{flagWidth, flagHeight}
		// Auto-size the window if appropriate.
		if flagAutoResize {
			log.Printf(">>> img[%s]...\n", names[0])
			b := imgs[0].Bounds()
			winSize = image.Point{b.Dx(), b.Dy()}
		}

		w, err := s.NewWindow(&screen.NewWindowOptions{
			Width:  winSize.X,
			Height: winSize.Y,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer w.Release()

		b, err := s.NewBuffer(winSize)
		if err != nil {
			log.Fatal(err)
		}
		defer b.Release()

		w.Fill(b.Bounds(), color.White, draw.Src)
		w.Publish()

		var sz size.Event
		var i int // index of image to display
		for e := range w.Events() {
			switch e := e.(type) {
			default:
				fmt.Printf("got %#v\n", e)

			case mouse.Event:
				fmt.Printf("got-mouse: %v\n", e)

			case key.Event:
				fmt.Printf("got-key-evt %v\n", e)
				repaint := false
				switch e.Code {
				case key.CodeEscape, key.CodeQ:
					return
				case key.CodeRightArrow:
					if e.Direction == key.DirPress {
						if i == len(imgs)-1 {
							i = -1
						}
						i++
						repaint = true
						b.Release()
						b, err = s.NewBuffer(sz.Size())
						if err != nil {
							log.Fatal(err)
						}
					}

				case key.CodeLeftArrow:
					if e.Direction == key.DirPress {
						if i == 0 {
							i = len(imgs)
						}
						i--
						repaint = true
						b, err = s.NewBuffer(sz.Size())
						if err != nil {
							log.Fatal(err)
						}
					}

				case key.CodeR:
					if e.Direction == key.DirPress {
						// resize to current image
						r := imgs[i].Bounds()
						sz.HeightPx = r.Dy()
						sz.WidthPx = r.Dx()
						repaint = true
						b, err = s.NewBuffer(sz.Size())
						if err != nil {
							log.Fatal(err)
						}
						w.Publish()

					}
				}
				if repaint {
					w.Send(paint.Event{})
				}

			case paint.Event:
				fmt.Printf("got %#v\n", e)
				img := imgs[i]
				fmt.Printf("-> %v | %v (idx=%d)\n", img.Bounds(), b.Bounds(), i)
				draw.Draw(b.RGBA(), b.Bounds(), img, image.Point{}, draw.Src)
				dp := vpCenter(img, sz.WidthPx, sz.HeightPx)
				zero := image.Point{}
				if dp != zero {
					w.Fill(sz.Bounds(), color.Black, draw.Src)
				}
				w.Upload(dp, b, b.Bounds(), w)
				fmt.Printf("<- %#v\n", w.Publish())

			case size.Event:
				fmt.Printf("resize-event: %v -> %v\n", sz, e)
				sz = e

			case error:
				log.Print(e)
			}
		}
	})
}

func findFiles(args []string) []string {
	files := []string{}
	for _, f := range args {
		fi, err := os.Stat(f)
		if err != nil {
			log.Print("Can't access", f, err)
		} else if fi.IsDir() {
			files = append(files, dirImages(f)...)
		} else {
			files = append(files, f)
		}
	}
	return files
}

func dirImages(dir string) []string {

	fd, _ := os.Open(dir)
	fs, _ := fd.Readdirnames(0)
	files := []string{}
	for _, f := range fs {
		// TODO filter by regexp
		if filepath.Ext(f) != "" {
			files = append(files, filepath.Join(dir, f))
		}
	}
	return files
}

// decodeImages takes a list of image files and decodes them into image.Image
// types. Note that the number of images returned may not be the number of
// image files passed in. Namely, an image file is skipped if it cannot be
// read or deocoded into an image type that Go understands.
func decodeImages(imageFiles []string) ([]string, []image.Image) {
	// A temporary type used to transport decoded images over channels.
	type tmpImage struct {
		img  image.Image
		name string
	}

	// Decoded all images specified in parallel.
	imgChans := make([]chan tmpImage, len(imageFiles))
	for i, fName := range imageFiles {
		imgChans[i] = make(chan tmpImage, 0)
		go func(i int, fName string) {
			file, err := os.Open(fName)
			if err != nil {
				log.Println(err)
				close(imgChans[i])
				return
			}

			start := time.Now()
			img, kind, err := image.Decode(file)
			if err != nil {
				log.Printf("Could not decode '%s' into a supported image "+
					"format: %s", fName, err)
				close(imgChans[i])
				return
			}
			log.Printf("Decoded '%s' into image type '%s' (%s).",
				fName, kind, time.Since(start))

			imgChans[i] <- tmpImage{
				img:  img,
				name: basename(fName),
			}
		}(i, fName)
	}

	// Now collect all the decoded images into a slice of names and a slice
	// of images.
	names := make([]string, 0, flag.NArg())
	imgs := make([]image.Image, 0, flag.NArg())
	for _, imgChan := range imgChans {
		if tmpImg, ok := <-imgChan; ok {
			names = append(names, tmpImg.name)
			imgs = append(imgs, tmpImg.img)
		}
	}

	return names, imgs
}

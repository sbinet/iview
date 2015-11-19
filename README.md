iview
=====

A minimalistic image viewer written in Go.

The list of image formats supported is currently at the mercy of whatever is 
supported by the Go standard library.

Currently, those formats are:

- `image/gif`
- `image/jpeg`
- `image/png`
- `golang.org/x/image/bmp`
- `golang.org/x/image/tiff`

Please see `iview -help` for more options.

## Quick Usage

```sh
$> go get github.com/sbinet/iview
$> iview image.png image.gif image.jpg
```

## Installation

```sh
$> go get github.com/sbinet/iview
```

## Acknowledgements

The original code base has been reaped off `github.com/BurntSushi/imgv`


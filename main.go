package main

import (
	"flag"
	"fmt"
	"github.com/bogem/id3v2"
	"github.com/mpetavy/common"
	"golang.org/x/exp/slices"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

const (
	titleSeparator = " - "
)

var (
	inputs    common.MultiValueFlag
	input     string
	output    *string
	recursive *bool
	dry       *bool
	various   *bool
	artists   []string
	track     int
)

func init() {
	common.Init(false, "1.0.0", "", "", "2018", "musicbrainz", "mpetavy", fmt.Sprintf("https://github.com/mpetavy/%s", common.Title()), common.APACHE, nil, nil, nil, run, 0)

	flag.Var(&inputs, "i", "input path(s)")
	output = flag.String("o", "", "output path")
	dry = flag.Bool("n", true, "dry run")
	various = flag.Bool("v", true, "various artists collection")
	recursive = flag.Bool("r", false, "recursive directory walk")
}

func fixString(s string) string {
	var sb strings.Builder

	for _, r := range []rune(s) {
		if unicode.IsDigit(r) || unicode.IsLetter(r) || unicode.IsSpace(r) {
			sb.WriteRune(r)
		}
	}

	n := sb.String()
	nLower := strings.ToLower(n)

	searches := []string{"various artists", "various"}
	for _, search := range searches {
		p := strings.Index(nLower, search)
		if p == -1 {
			continue
		}

		n = n[:p] + n[p+len(search):]
		nLower = nLower[:p] + nLower[p+len(search):]
	}

	return strings.ReplaceAll(strings.Title(strings.ToLower(strings.TrimSpace(n))), "  ", " ")
}

func processFile(filename string, f os.FileInfo) error {
	if f.IsDir() {
		track = 0
	}

	if f.IsDir() || !strings.HasSuffix(filename, ".mp3") {
		return nil
	}

	tag, err := id3v2.Open(filename, id3v2.Options{Parse: true})
	if common.Warn(err) {
		return nil
	}
	defer func() {
		common.Error(tag.Close())
	}()

	if tag.Artist() == "AC/DC" {
		fmt.Printf("stop\n")
	}

	album, _ := filepath.Split(filename)
	album = fixString(filepath.Base(album))

	artist := fixString(tag.Artist())
	if artist == "" {
		p := strings.Index(filepath.Base(filename), "-")
		if p != -1 {
			artist = fixString(filepath.Base(filename)[:p])
		}
	}

	if !slices.Contains(artists, artist) {
		artists = append(artists, artist)
	}

	title := strings.ReplaceAll(tag.Title(), tag.Artist(), "")
	title = strings.ReplaceAll(title, "-", "")
	title = fixString(title)
	if title == "" {
		p := strings.Index(filepath.Base(filename), "-")
		if p != -1 {
			fn := fixString(filepath.Base(filename)[p+1:])
			fn = fn[:len(fn)-3]
			title = fn
		}
	}

	var (
		tagArtist string
		tagAlbum  string
		tagTitle  string
	)

	if *various {
		tagArtist = "Various Artists"
		tagAlbum = album
		tagTitle = artist + titleSeparator + title
	} else {
		tagArtist = artist
		tagAlbum = album
		tagTitle = title
	}

	targetFile := filepath.Join(*output, tagArtist, tagAlbum, fmt.Sprintf("%03d %s.mp3", track, tagTitle))

	if !*dry {
		targetPath := filepath.Dir(targetFile)

		if !common.FileExists(targetPath) {
			err := os.MkdirAll(targetPath, common.DefaultDirMode)
			if common.Error(err) {
				return err
			}
		}

		err := common.FileCopy(filename, targetFile)
		if common.Error(err) {
			return err
		}

		tag, err = id3v2.Open(targetFile, id3v2.Options{Parse: true})
		if common.Warn(err) {
			return nil
		}
		defer func() {
			common.Error(tag.Close())
		}()
	}

	track++

	tag.SetArtist(tagArtist)
	tag.SetAlbum(tagAlbum)
	tag.SetTitle(tagTitle)
	tag.AddTextFrame(tag.CommonID("Track number/Position in set"), tag.DefaultEncoding(), strconv.Itoa(track))

	if !*dry && common.Warn(tag.Save()) {
		return nil
	}

	if *dry {
		fmt.Printf("- would... ------------------\n")
	} else {
		fmt.Printf("- will... -------------------\n")
	}

	fmt.Printf("filename:     %s\n", filename)
	fmt.Printf("new filename: %s\n", targetFile)
	fmt.Printf("artist:       %s\n", tag.Artist())
	fmt.Printf("album:        %s\n", tag.Album())
	fmt.Printf("track:        %s\n", strconv.Itoa(track))
	fmt.Printf("title:        %s\n", tag.Title())

	return nil
}

func scanPath(path string) error {
	fw := common.NewFilewalker(path, *recursive, true, processFile)

	err := fw.Run()
	if common.Error(err) {
		return err
	}

	slices.Sort(artists)

	return nil
}

func run() error {
	for _, input = range inputs {
		err := scanPath(input)
		if common.Error(err) {
			return err
		}
	}

	return nil
}

func main() {
	defer common.Done()

	common.Run([]string{"i"})
}

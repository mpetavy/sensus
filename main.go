package main

import (
	"flag"
	"fmt"
	"github.com/bogem/id3v2"
	"github.com/mpetavy/common"
	"golang.org/x/exp/slices"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

func run1() error {
	input := "/home/ransom/output/Various Artists/Pumuckel"
	output := "/home/ransom/output/Various Artists/Pumuckl"

	readFiles, err := os.ReadDir(input)
	if common.Error(err) {
		return err
	}

	files := []string{}
	for _, file := range readFiles {
		if file.IsDir() {
			continue
		}

		files = append(files, file.Name())
	}

	sort.Strings(files)

	dups := make(map[string]int)

	for i, file := range files {
		from := filepath.Join(input, file)
		to := file

		searches := []string{
			"Kinder Hörspiel .*$",
			"Mirabelle B \\- ",
			"Meister Eder Und Sein Pumuckl ",
			"Meister Eder Und Sein Hörbuch",
			"Meister Eder Und Sein",
			" Deutsch",
			" Kobold",
			"Pumuckl Und ",
			"Hörspiel",
			"Für Kinder",
			"Folge",
			"Cd",
			"Lp",
			"Mc",
			"Ellis",
			"Audiobook",
			"Kaut",
			"Und Seine n ",
			".mp3",
			" 1  ",
			"\\d *",
		}

		for _, search := range searches {
			r, err := regexp.Compile(search)
			if common.Error(err) {
				return err
			}

			loc := r.FindStringIndex(to)
			if loc == nil {
				continue
			}

			to = to[:loc[0]] + to[loc[1]:]
		}

		to = strings.TrimSpace(to)
		to = fmt.Sprintf("%02d %s", i+1, to[3:])
		to = strings.ReplaceAll(to, "  ", " ")
		to = filepath.Join(output, strings.TrimSpace(to))

		r, err := regexp.Compile("[\\wöäüÖÄÜß]*")
		if common.Error(err) {
			return err
		}

		words := r.FindAllString(to[3:], -1)
		for _, word := range words {
			if len(word) == 0 {
				continue
			}

			v, _ := dups[word]
			v++
			dups[word]++
		}

		err = common.FileCopy(from, to)
		if common.Error(err) {
			return err
		}

		tag, err := id3v2.Open(to, id3v2.Options{Parse: true})
		if common.Warn(err) {
			return nil
		}

		tag.SetArtist("Various Artists")
		tag.SetAlbum("Pumuckl")
		tag.SetAlbum("Meister Eder Und sein Pumuckl")
		tag.SetTitle(to)

		err = tag.Save()
		if common.Error(err) {
			return nil
		}

		common.Error(tag.Close())

		fmt.Printf("---------------------\n")
		fmt.Printf("%s\n", from)
		fmt.Printf("%s\n", to)
	}

	//var words []string
	//
	//for word, _ := range dups {
	//	words = append(words, word)
	//}
	//
	//sort.SliceStable(words, func(i, j int) bool {
	//	v0 := dups[words[i]]
	//	v1 := dups[words[j]]
	//
	//	return v0 > v1
	//
	//})
	//
	//for _, word := range words {
	//	fmt.Printf("%s\t\t\t%d\n", word, dups[word])
	//}

	return nil
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
	common.Error(tag.Close())

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
		if artist == album {
			tagTitle = title
		} else {
			tagTitle = artist + titleSeparator + title
		}
	} else {
		tagArtist = artist
		tagAlbum = album
		tagTitle = title
	}

	track++

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
	}

	tag.SetArtist(tagArtist)
	tag.SetAlbum(tagAlbum)
	tag.SetTitle(tagTitle)
	tag.AddTextFrame(tag.CommonID("Track number/Position in set"), tag.DefaultEncoding(), strconv.Itoa(track))

	if !*dry && common.Warn(tag.Save()) {
		return nil
	}

	common.Error(tag.Close())

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

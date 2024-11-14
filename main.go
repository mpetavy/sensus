package main

import (
	"embed"
	"flag"
	"fmt"
	"github.com/bogem/id3v2"
	"github.com/mpetavy/common"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	PICTURE = "APIC"
	ALBUM   = "TALB"
	TITLE   = "TIT2"
	ARTIST  = "TPE1"
	TRACK   = "TRCK"
)

var (
	input              = flag.String("i", "", "input path")
	output             = flag.String("o", "", "output path")
	verbose            = flag.Bool("v", false, "verbose MP3 tag info")
	recursive          = flag.Bool("r", false, "recursive directory walk")
	removeObsoleteTags = flag.Bool("x", false, "remove obsolete tags")
	updates            common.MultiValueFlag
	id3v2TagToDescs    = make(map[string]string)
	defaultTags        = []string{
		ALBUM,   // Album/Movie/Show title - Pumuckl
		ARTIST,  // Lead artist/Lead performer/Soloist/Performing group - Various Artists
		TRACK,   // Track number/Position in set - Track Number
		TITLE,   // TITLE - Der verdrehte Tag
		PICTURE, // Attached picture - Bild
	}
	track int
)

//go:embed go.mod
var resources embed.FS

func init() {
	common.Init("", "", "", "", "musicbrainz", "", "", "", &resources, nil, nil, run, 0)

	flag.Var(&updates, "u", "MP3 to update")
}

func createTarget(filename string, source string, target string) string {
	filename = common.CleanPath(filename)
	source = common.CleanPath(source)
	target = common.CleanPath(target)

	return filepath.Join(target, filename[len(source):])
}

func showTags(filename string, tags *id3v2.Tag) {
	sortedTags := make([]string, 0)
	for k := range tags.AllFrames() {
		sortedTags = append(sortedTags, k)
	}

	sort.Strings(sortedTags)

	if *verbose {
		st := common.NewStringTable()
		st.AddCols("file", "tag", "description", "data")

		for _, k := range sortedTags {
			frame := tags.GetLastFrame(k)
			t := fmt.Sprintf("%v", frame)
			if len(t) > 60 {
				t = t[:60] + "..."
			}

			st.AddCols(filename[len(*input)+1:], k, id3v2TagToDescs[k], t)
		}

		fmt.Printf("%s\n", st.Table())
	} else {
		fmt.Printf("%s\n", filepath.Base(filename))
	}

}

func processFile(filename string, f os.FileInfo) error {
	if f.IsDir() {
		fmt.Println()
		fmt.Printf("[%s]\n", filename)

		track = 0
	}

	if f.IsDir() || !strings.HasSuffix(filename, ".mp3") {
		return nil
	}

	var tags *id3v2.Tag

	if *output != "" {
		targetFile := createTarget(filename, *input, *output)
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

		filename = targetFile
	}

	options := id3v2.Options{Parse: true}

	if *removeObsoleteTags {
		options.ParseFrames = defaultTags
	}

	tags, err := id3v2.Open(filename, options)
	if common.Error(err) {
		tags, err = id3v2.Open(filename, id3v2.Options{Parse: false})
		if common.Error(err) {
			return nil
		}
	}

	defer func() {
		common.Error(tags.Close())
	}()

	tags.SetVersion(3)

	if *output == "" {
		showTags(filename, tags)

		return nil
	}

	track++

	tags.AddTextFrame(TRACK, tags.DefaultEncoding(), strconv.Itoa(track))

	newName := ""

	for _, update := range updates {
		splits := strings.Split(update, "=")
		if len(splits) != 2 {
			return fmt.Errorf("wrong update: %s", update)
		}

		tag := splits[0]
		value := splits[1]

		_, ok := id3v2TagToDescs[tag]

		if !ok {
			return fmt.Errorf("invalid tag: %s", tag)
		}

		if value == "" {
			tags.DeleteFrames(tag)

			continue
		}

		if strings.HasPrefix(value, "~") {
			_, ok = id3v2TagToDescs[value[1:]]

			if !ok {
				return fmt.Errorf("invalid tag: %s", tag)
			}

			value = tags.GetTextFrame(value[1:]).Text
		}

		if tag == TITLE {
			newName = value
		}

		tags.AddTextFrame(tag, tags.DefaultEncoding(), value)
	}

	showTags(filename, tags)

	err = tags.Save()
	if common.Error(err) {
		return nil
	}

	if newName != "" {
		err = os.Rename(filename, filepath.Join(filepath.Dir(filename), newName))
		if common.Error(err) {
			return nil
		}
	}

	return nil
}

func scanPath(path string) error {
	err := common.WalkFiles(path, *recursive, true, processFile)
	if common.Error(err) {
		return err
	}

	return nil
}

func run() error {
	if *input != "" {
		*input = common.CleanPath(*input)
	}
	if *output != "" {
		*output = common.CleanPath(*output)
	}

	if *output != "" {
		if !common.FileExists(*output) {
			common.Panic(fmt.Errorf("output path does not exist: %s", *output))
		}

		if !common.IsDirectory(*output) {
			common.Panic(fmt.Errorf("output path is not a directory: %s", *output))
		}
	}

	for k, v := range id3v2.V23CommonIDs {
		id3v2TagToDescs[v] = k
	}

	err := scanPath(*input)
	if common.Error(err) {
		return err
	}

	return nil
}

func main() {
	common.Run([]string{"i"})
}

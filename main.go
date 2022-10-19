package main

import (
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
	dry                = flag.Bool("n", false, "dry run")
	recursive          = flag.Bool("r", false, "recursive directory walk")
	removeObsoleteTags = flag.Bool("x", false, "remove obsolete tags")
	updates            common.MultiValueFlag
	id3v2Tags          = make(map[string]string)
	defaultTags        = []string{
		PICTURE, // Attached picture - Bild
		ALBUM,   // Album/Movie/Show title - Pumuckl
		TITLE,   // TITLE - Der verdrehte Tag
		ARTIST,  // Lead artist/Lead performer/Soloist/Performing group - Various Artists
		TRACK,   // Track number/Position in set - Track Number
	}
	track int
)

func init() {
	common.Init(false, "1.0.0", "", "", "2018", "musicbrainz", "mpetavy", fmt.Sprintf("https://github.com/mpetavy/%s", common.Title()), common.APACHE, nil, nil, nil, run, 0)

	flag.Var(&updates, "u", "MP3 to update")

	common.Events.NewFuncReceiver(common.EventFlagsParsed{}, func(event common.Event) {
		if *output == "" {
			return
		}

		if !common.FileExists(*output) {
			common.Panic(fmt.Errorf("output path does not exist: %s", *output))
		}

		if !common.IsDirectory(*output) {
			common.Panic(fmt.Errorf("output path is not a directory: %s", *output))
		}
	})

	for k, v := range id3v2.V23CommonIDs {
		id3v2Tags[v] = k
	}
}

func createTarget(filename string, source string, target string) string {
	filename = common.CleanPath(filename)
	source = common.CleanPath(source)
	target = common.CleanPath(target)

	if filename == source {
		return filepath.Join(target, filepath.Base(filename))
	}

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

			st.AddCols(filename, k, id3v2Tags[k], t)
		}

		fmt.Printf("%s\n", st.String())
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

	targetFile := filename
	if *output != "" {
		targetFile = createTarget(filename, *input, *output)
	}

	var tags *id3v2.Tag

	if !*dry && *output != "" {
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

		options := id3v2.Options{Parse: true}

		if *removeObsoleteTags {
			options.ParseFrames = defaultTags
		}

		tags, err = id3v2.Open(targetFile, options)
		if common.Error(err) {
			tags, err = id3v2.Open(filename, id3v2.Options{Parse: false})
			if common.Error(err) {
				return nil
			}
		}
	} else {
		var err error

		tags, err = id3v2.Open(filename, id3v2.Options{Parse: true})
		if common.Error(err) {
			tags, err = id3v2.Open(filename, id3v2.Options{Parse: true})
			if common.Error(err) {
				return nil
			}
		}
	}

	tags.SetVersion(3)

	showTags(filename, tags)

	if *output == "" {
		common.Error(tags.Close())

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

		_, ok := id3v2Tags[tag]

		if !ok {
			return fmt.Errorf("invalid tag: %s", tag)
		}

		if tag == TITLE {
			newName = value
		}

		tags.AddTextFrame(tag, tags.DefaultEncoding(), value)
	}

	showTags(targetFile, tags)

	if *dry {
		return nil
	}

	err := tags.Save()
	if common.Error(err) {
		return nil
	}

	common.Error(tags.Close())

	if newName != "" {
		err = os.Rename(targetFile, filepath.Join(filepath.Dir(targetFile), newName))
		if common.Error(err) {
			return nil
		}
	}

	return nil
}

func scanPath(path string) error {
	fw := common.NewFilewalker(path, *recursive, true, processFile)

	err := fw.Run()
	if common.Error(err) {
		return err
	}

	return nil
}

func run() error {
	err := scanPath(*input)
	if common.Error(err) {
		return err
	}

	return nil
}

func main() {
	defer common.Done()

	common.Run([]string{"i"})
}

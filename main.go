package main

import (
	"flag"
	"fmt"
	"github.com/bogem/id3v2"
	"github.com/mpetavy/common"
	"os"
)

var (
	paths common.MultiValueFlag
)

func init() {
	common.Init(false, "1.0.0", "", "", "2018", "musicbrainz", "mpetavy", fmt.Sprintf("https://github.com/mpetavy/%s", common.Title()), common.APACHE, nil, nil, nil, run, 0)

	flag.Var(&paths, "p", "Path tp search for MP3 folders")
}

func processFile(filename string, f os.FileInfo) error {
	tag, err := id3v2.Open(filename, id3v2.Options{Parse: true})
	if common.Error(err) {
		return err
	}
	defer tag.Close()

	// Read tags.
	fmt.Printf("%s | %s | %s | %s\n", tag.GetFrames(tag.CommonID("Track number/Position in set")), tag.Artist(), tag.Album(), tag.Title())

	return nil

	// Set tags.
	tag.SetArtist("Aphex Twin")
	tag.SetTitle("Xtal")

	comment := id3v2.CommentFrame{
		Encoding:    id3v2.EncodingUTF8,
		Language:    "eng",
		Description: "My opinion",
		Text:        "I like this song!",
	}
	tag.AddCommentFrame(comment)

	// Write tag to file.mp3.
	//if err = tag.Save(); err != nil {
	//	log.Fatal("Error while saving a tag: ", err)
	//}

	return nil
}

func scanPath(path string) error {
	fw := common.NewFilewalker(path, false, true, processFile)

	err := fw.Run()
	if common.Error(err) {
		return err
	}

	return nil
}

func run() error {
	for _, path := range paths {
		err := scanPath(path)
		if common.Error(err) {
			return err
		}
	}

	return nil
}

func main() {
	defer common.Done()

	common.Run([]string{"p"})
}

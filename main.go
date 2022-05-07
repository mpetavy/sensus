package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/bogem/id3v2"
	"github.com/michiwend/gomusicbrainz"
	"github.com/mpetavy/common"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"time"
)

const (
	CdStubFile = "cdstub.json"
)

var (
	paths       common.MultiValueFlag
	paceTimeout = flag.Int("pt", 10, "pace timeout in msec for Musicbrainz API request")
	lastCall    = time.Time{}
	client      *gomusicbrainz.WS2Client
)

func init() {
	common.Init(false, "1.0.0", "", "", "2018", "musicbrainz", "mpetavy", fmt.Sprintf("https://github.com/mpetavy/%s", common.Title()), common.APACHE, nil, nil, nil, run, 0)

	flag.Var(&paths, "p", "Path tp search for MP3 folders")
}

func run0() error {
	tag, err := id3v2.Open("d:\\temp\\04___BOOGIEMAN.MP3", id3v2.Options{Parse: true})
	if err != nil {
		log.Fatal("Error while opening mp3 file: ", err)
	}
	defer tag.Close()

	// Read tags.
	fmt.Println(tag.Artist())
	fmt.Println(tag.Album())
	fmt.Println(tag.Title())

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
	if err = tag.Save(); err != nil {
		log.Fatal("Error while saving a tag: ", err)
	}

	return nil
}

func run1() error {
	var err error
	client, err = gomusicbrainz.NewWS2Client(
		"https://musicbrainz.org/ws/2",
		"A GoMusicBrainz example",
		"0.0.1-beta",
		"http://github.com/michiwend/gomusicbrainz")
	if common.Error(err) {
		return err
	}

	//respCd, err := client.SearchCDStub("title:'...Ish!'", -1, -1)
	respCd, err := client.SearchCDStub("title:Bochum", -1, -1)
	if common.Error(err) {
		return err
	}

	st := common.StringTable{}
	for _, cdstub := range respCd.CDStubs {
		st.AddCols(cdstub.ID, cdstub.Artist, cdstub.Title)
	}

	fmt.Printf("%s\n", st.String())

	respArtist, err := client.SearchArtist(`artist:"ISH"`, -1, -1)
	if common.Error(err) {
		return err
	}

	var mbid gomusicbrainz.MBID

	st = common.StringTable{}
	st.AddCols("Id", "Name", "Score")
	for _, artist := range respArtist.Artists {
		st.AddCols(artist.Id, artist.Name, respArtist.Scores[artist])

		mbid = artist.ID
	}

	// ---------------

	artist, err := client.LookupArtist(mbid)
	if common.Error(err) {
		return err
	}

	fmt.Printf("%+v\n", artist)

	// ---------------

	respCDStub, err := client.SearchRecording("1927", -1, -1)
	if common.Error(err) {
		return err
	}

	st.Clear()
	st.AddCols("Id", "Title", "ArtistCredit", "Score")
	for _, recording := range respCDStub.Recordings {
		st.AddCols(recording.ID, recording.Title, recording.ArtistCredit.NameCredits[0].Artist.ID, respCDStub.Scores[recording])
	}

	fmt.Printf("%s\n", st.String())

	return nil
}

type CD struct {
	Id     gomusicbrainz.MBID
	Title  string
	Artist string
}

func scanPath(path string) error {
	files, err := ioutil.ReadDir(path)
	if common.Error(err) {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		err := scanCDPath(filepath.Join(path, file.Name()))
		if common.Error(err) {
			return err
		}

		return nil
	}

	return nil
}

func paceClient() {
	if lastCall.IsZero() {
		return
	}

	sleep := lastCall.Add(common.MillisecondToDuration(*paceTimeout)).Sub(time.Now())
	if sleep < 0 {
		return
	}

	time.Sleep(time.Millisecond * sleep)
}

func scanCDPath(path string) error {
	fmt.Printf("%s\n", path)

	paceClient()

	title := filepath.Base(path)
	p := strings.Index(title, "-")
	if p != -1 {
		title = title[p+1:]
		title = strings.TrimSpace(title)
	}

	respCd, err := client.SearchCDStub(title, -1, -1)
	if common.Error(err) {
		return err
	}

	for _, cdstub := range respCd.CDStubs {
		ba, err := json.MarshalIndent(cdstub, "", "  ")
		if common.Error(err) {
			return err
		}
		fmt.Printf("%s\n", string(ba))

		ioutil.WriteFile(filepath.Dir(path), ba, common.DefaultFileMode)
	}

	fmt.Println()

	return nil
}

func run() error {
	var err error

	client, err = gomusicbrainz.NewWS2Client(
		"https://musicbrainz.org/ws/2",
		common.App().Name,
		common.App().Version,
		common.App().Homepage)
	if common.Error(err) {
		return err
	}

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
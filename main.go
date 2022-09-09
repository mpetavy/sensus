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
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	CDInfoFile  = "cd-info.json"
	CDInfosFile = "cd-infos.json"
)

type CDInfo struct {
	Path      string                 `json:"path"`
	Search    string                 `json:"search"`
	FileCount int                    `json:"fileCount"`
	CDStub    gomusicbrainz.CDStub   `json:"cdStub"`
	CDStubs   []gomusicbrainz.CDStub `json:"cdStubs"`
}

var (
	paths       common.MultiValueFlag
	refresh     = flag.Bool("r", false, "refresh CD infos")
	paceTimeout = flag.Int("pt", 100, "pace timeout in msec for Musicbrainz API request")
	lastCall    = time.Time{}
	client      *gomusicbrainz.WS2Client
	cdInfos     []CDInfo
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

func scanPath(path string) error {
	common.DebugFunc(path)

	files, err := ioutil.ReadDir(path)
	if common.Error(err) {
		return err
	}

	sort.SliceStable(files, func(i int, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		err := scanCDPath(filepath.Join(path, file.Name()))
		if common.Error(err) {
			return err
		}

		//FIXME
		if len(cdInfos) == 10 {
			break
		}
	}

	return nil
}

func paceClient() {
	common.DebugFunc()

	if lastCall.IsZero() {
		return
	}

	sleep := lastCall.Add(common.MillisecondToDuration(*paceTimeout)).Sub(time.Now())
	if sleep < 0 {
		return
	}

	time.Sleep(time.Millisecond * sleep)
}

func writeCDInfo(path string, cdInfo *CDInfo) error {
	common.DebugFunc(path)

	fn := filepath.Join(path, CDInfoFile)

	ba, err := json.MarshalIndent(cdInfo, "", "    ")
	if common.Error(err) {
		return err
	}

	err = os.WriteFile(fn, ba, common.DefaultFileMode)
	if common.Error(err) {
		return err
	}

	return err
}

func deleteCDInfo(path string) error {
	common.DebugFunc(path)

	fn := filepath.Join(path, CDInfoFile)

	err := common.FileDelete(fn)
	if common.Error(err) {
		return err
	}

	return nil
}

func readCDInfo(path string) (*CDInfo, error) {
	common.DebugFunc(path)

	fn := filepath.Join(path, CDInfoFile)

	if !common.FileExists(fn) {
		return &CDInfo{Path: path}, nil
	}

	ba, err := os.ReadFile(fn)
	if common.Error(err) {
		return nil, err
	}

	cdInfo := &CDInfo{}

	err = json.Unmarshal(ba, cdInfo)
	if common.Error(err) {
		return nil, err
	}

	return cdInfo, nil
}

func queryCDStubs(path string, cdInfo *CDInfo) error {
	common.DebugFunc(path)

	paceClient()

	artist := filepath.Base(path)
	p := strings.Index(artist, " - ")
	if p != -1 {
		artist = artist[:p]
		artist = strings.TrimSpace(artist)
	}

	title := filepath.Base(path)
	p = strings.LastIndex(title, " - ")
	if p != -1 {
		title = title[p+3:]
		title = strings.TrimSpace(title)
	}

	var search string

	if len(artist) > 0 {
		search += fmt.Sprintf("artist:'%s'", artist)
	}
	if len(title) > 0 {
		if len(search) > 0 {
			search = search + " AND "
		}
		search += fmt.Sprintf("title:'%s'", title)
	}

	cdInfo.Search = search

	files, err := ioutil.ReadDir(path)
	if common.Error(err) {
		return err
	}

	fileCount := 0
	for _, file := range files {
		if !file.IsDir() && file.Name() != CDInfoFile {
			fileCount++
		}
	}

	cdInfo.FileCount = fileCount

	respCd, err := client.SearchCDStub(title, -1, -1)
	if common.Error(err) {
		return err
	}

	for i, cdstub := range respCd.CDStubs {
		if i == 0 {
			cdInfo.CDStub = *cdstub
		}

		cdInfo.CDStubs = append(cdInfo.CDStubs, *cdstub)

		//if cdInfo.CDStub.TrackList.Count != fileCount && fileCount == cdstub.TrackList.Count {
		//	cdInfo.CDStub = *cdstub
		//}
	}

	return nil
}

func scanCDPath(path string) error {
	common.DebugFunc(path)

	if *refresh {
		err := deleteCDInfo(path)
		if common.Error(err) {
			return err
		}
	}

	cdInfo, err := readCDInfo(path)
	if common.Error(err) {
		return err
	}

	if cdInfo.CDStubs == nil {
		queryCDStubs(path, cdInfo)
	}

	err = writeCDInfo(path, cdInfo)
	if common.Error(err) {
		return err
	}

	cdInfos = append(cdInfos, *cdInfo)

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

		st := common.NewStringTable(false)
		st.AddRow()
		st.AddCol("Path")
		st.AddCol("Search")
		st.AddCol("FileCount")
		st.AddCol("Artist")
		st.AddCol("Title")
		st.AddCol("Traclist-Count")

		for _, cdInfo := range cdInfos {
			st.AddRow()
			st.AddCol(filepath.Base(cdInfo.Path))
			st.AddCol(cdInfo.Search)
			st.AddCol(cdInfo.FileCount)
			st.AddCol(cdInfo.CDStub.Artist)
			st.AddCol(cdInfo.CDStub.Title)
			st.AddCol(cdInfo.CDStub.TrackList.Count)
		}

		fmt.Printf("%s\n", st.String())

		os.WriteFile(filepath.Join(path, CDInfosFile), []byte(st.String()), common.DefaultFileMode)
	}

	return nil
}

func main() {
	defer common.Done()

	common.Run([]string{"p"})
}

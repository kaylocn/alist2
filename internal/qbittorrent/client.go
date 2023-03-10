package qbittorrent

import (
	"bytes"
	"errors"
	"github.com/alist-org/alist/v3/pkg/utils"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

type Client interface {
	AddFromLink(link string, savePath string, id string) error
	GetInfo(id string) (TorrentInfo, error)
	GetFiles(id string) ([]FileInfo, error)
	Delete(id string) error
}

type client struct {
	url    *url.URL
	client http.Client
	Client
}

func New(webuiUrl string) (Client, error) {
	u, err := url.Parse(webuiUrl)
	if err != nil {
		return nil, err
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	var c = &client{
		url:    u,
		client: http.Client{Jar: jar},
	}

	err = c.checkAuthorization()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *client) checkAuthorization() error {
	// check authorization
	if c.authorized() {
		return nil
	}

	// check authorization after logging in
	err := c.login()
	if err != nil {
		return err
	}
	if c.authorized() {
		return nil
	}
	return errors.New("unauthorized qbittorrent url")
}

func (c *client) authorized() bool {
	resp, err := c.post("/api/v2/app/version", nil)
	if err != nil {
		return false
	}
	return resp.StatusCode == 200 // the status code will be 403 if not authorized
}

func (c *client) login() error {
	// prepare HTTP request
	v := url.Values{}
	v.Set("username", c.url.User.Username())
	passwd, _ := c.url.User.Password()
	v.Set("password", passwd)
	resp, err := c.post("/api/v2/auth/login", v)
	if err != nil {
		return err
	}

	// check result
	body := make([]byte, 2)
	_, err = resp.Body.Read(body)
	if err != nil {
		return err
	}
	if string(body) != "Ok" {
		return errors.New("failed to login into qBittorrent webui with url: " + c.url.String())
	}
	return nil
}

func (c *client) post(path string, data url.Values) (*http.Response, error) {
	u := c.url.JoinPath(path)
	u.User = nil // remove userinfo for requests

	req, err := http.NewRequest("POST", u.String(), bytes.NewReader([]byte(data.Encode())))
	if err != nil {
		return nil, err
	}
	if data != nil {
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.Cookies() != nil {
		c.client.Jar.SetCookies(u, resp.Cookies())
	}
	return resp, nil
}

func (c *client) AddFromLink(link string, savePath string, id string) error {
	err := c.checkAuthorization()
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)

	addField := func(name string, value string) {
		if err != nil {
			return
		}
		err = writer.WriteField(name, value)
	}
	addField("urls", link)
	addField("savepath", savePath)
	addField("tags", "alist-"+id)
	if err != nil {
		return err
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	u := c.url.JoinPath("/api/v2/torrents/add")
	u.User = nil // remove userinfo for requests
	req, err := http.NewRequest("POST", u.String(), buf)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	// check result
	body := make([]byte, 2)
	_, err = resp.Body.Read(body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 || string(body) != "Ok" {
		return errors.New("failed to add qBittorrent task: " + link)
	}
	return nil
}

type TorrentStatus string

const (
	ERROR              TorrentStatus = "error"
	MISSINGFILES       TorrentStatus = "missingFiles"
	UPLOADING          TorrentStatus = "uploading"
	PAUSEDUP           TorrentStatus = "pausedUP"
	QUEUEDUP           TorrentStatus = "queuedUP"
	STALLEDUP          TorrentStatus = "stalledUP"
	CHECKINGUP         TorrentStatus = "checkingUP"
	FORCEDUP           TorrentStatus = "forcedUP"
	ALLOCATING         TorrentStatus = "allocating"
	DOWNLOADING        TorrentStatus = "downloading"
	METADL             TorrentStatus = "metaDL"
	PAUSEDDL           TorrentStatus = "pausedDL"
	QUEUEDDL           TorrentStatus = "queuedDL"
	STALLEDDL          TorrentStatus = "stalledDL"
	CHECKINGDL         TorrentStatus = "checkingDL"
	FORCEDDL           TorrentStatus = "forcedDL"
	CHECKINGRESUMEDATA TorrentStatus = "checkingResumeData"
	MOVING             TorrentStatus = "moving"
	UNKNOWN            TorrentStatus = "unknown"
)

// https://github.com/DGuang21/PTGo/blob/main/app/client/client_distributer.go
type TorrentInfo struct {
	AddedOn           int           `json:"added_on"`           // ??? torrent ??????????????????????????????Unix Epoch???
	AmountLeft        int64         `json:"amount_left"`        // ????????????????????????
	AutoTmm           bool          `json:"auto_tmm"`           // ??? torrent ????????? Automatic Torrent Management ??????
	Availability      float64       `json:"availability"`       // ???????????????
	Category          string        `json:"category"`           //
	Completed         int64         `json:"completed"`          // ????????????????????????????????????
	CompletionOn      int           `json:"completion_on"`      // Torrent ??????????????????Unix Epoch???
	ContentPath       string        `json:"content_path"`       // torrent ????????????????????????????????? torrent ???????????????????????? torrent ????????????????????????
	DlLimit           int           `json:"dl_limit"`           // Torrent ???????????????????????????/??????
	Dlspeed           int           `json:"dlspeed"`            // Torrent ?????????????????????/??????
	Downloaded        int64         `json:"downloaded"`         // ??????????????????
	DownloadedSession int64         `json:"downloaded_session"` // ???????????????????????????
	Eta               int           `json:"eta"`                //
	FLPiecePrio       bool          `json:"f_l_piece_prio"`     // ???????????????????????????????????????????????????true
	ForceStart        bool          `json:"force_start"`        // ???????????? torrent ??????????????????????????????true
	Hash              string        `json:"hash"`               //
	LastActivity      int           `json:"last_activity"`      // ????????????????????????Unix Epoch???
	MagnetURI         string        `json:"magnet_uri"`         // ?????? torrent ????????? Magnet URI
	MaxRatio          int           `json:"max_ratio"`          // ??????/??????????????????????????????????????????
	MaxSeedingTime    int           `json:"max_seeding_time"`   // ???????????????????????????????????????????????????
	Name              string        `json:"name"`               //
	NumComplete       int           `json:"num_complete"`       //
	NumIncomplete     int           `json:"num_incomplete"`     //
	NumLeechs         int           `json:"num_leechs"`         // ???????????? leechers ?????????
	NumSeeds          int           `json:"num_seeds"`          // ?????????????????????
	Priority          int           `json:"priority"`           // ??????????????????????????????????????? torrent ?????????????????????????????? -1
	Progress          float64       `json:"progress"`           // ??????
	Ratio             float64       `json:"ratio"`              // Torrent ????????????
	RatioLimit        int           `json:"ratio_limit"`        //
	SavePath          string        `json:"save_path"`
	SeedingTime       int           `json:"seeding_time"`       // Torrent ?????????????????????
	SeedingTimeLimit  int           `json:"seeding_time_limit"` // max_seeding_time
	SeenComplete      int           `json:"seen_complete"`      // ?????? torrent ???????????????
	SeqDl             bool          `json:"seq_dl"`             // ?????????????????????????????????true
	Size              int64         `json:"size"`               //
	State             TorrentStatus `json:"state"`              // ??????https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)#get-torrent-list
	SuperSeeding      bool          `json:"super_seeding"`      // ?????????????????????????????????true
	Tags              string        `json:"tags"`               // Torrent ???????????????????????????
	TimeActive        int           `json:"time_active"`        // ????????????????????????
	TotalSize         int64         `json:"total_size"`         // ??? torrent ?????????????????????????????????????????????????????????????????????
	Tracker           string        `json:"tracker"`            // ??????????????????????????????tracker???????????????tracker????????????????????????????????????
	TrackersCount     int           `json:"trackers_count"`     //
	UpLimit           int           `json:"up_limit"`           // ????????????
	Uploaded          int64         `json:"uploaded"`           // ????????????
	UploadedSession   int64         `json:"uploaded_session"`   // ??????session????????????
	Upspeed           int           `json:"upspeed"`            // ?????????????????????/??????
}

type InfoNotFoundError struct {
	Id  string
	Err error
}

func (i InfoNotFoundError) Error() string {
	return "there should be exactly one task with tag \"alist-" + i.Id + "\""
}

func NewInfoNotFoundError(id string) InfoNotFoundError {
	return InfoNotFoundError{Id: id}
}

func (c *client) GetInfo(id string) (TorrentInfo, error) {
	var infos []TorrentInfo

	err := c.checkAuthorization()
	if err != nil {
		return TorrentInfo{}, err
	}

	v := url.Values{}
	v.Set("tag", "alist-"+id)
	response, err := c.post("/api/v2/torrents/info", v)
	if err != nil {
		return TorrentInfo{}, err
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return TorrentInfo{}, err
	}
	err = utils.Json.Unmarshal(body, &infos)
	if err != nil {
		return TorrentInfo{}, err
	}
	if len(infos) != 1 {
		return TorrentInfo{}, NewInfoNotFoundError(id)
	}
	return infos[0], nil
}

type FileInfo struct {
	Index        int     `json:"index"`
	Name         string  `json:"name"`
	Size         int64   `json:"size"`
	Progress     float32 `json:"progress"`
	Priority     int     `json:"priority"`
	IsSeed       bool    `json:"is_seed"`
	PieceRange   []int   `json:"piece_range"`
	Availability float32 `json:"availability"`
}

func (c *client) GetFiles(id string) ([]FileInfo, error) {
	var infos []FileInfo

	err := c.checkAuthorization()
	if err != nil {
		return []FileInfo{}, err
	}

	tInfo, err := c.GetInfo(id)
	if err != nil {
		return []FileInfo{}, err
	}

	v := url.Values{}
	v.Set("hash", tInfo.Hash)
	response, err := c.post("/api/v2/torrents/files", v)
	if err != nil {
		return []FileInfo{}, err
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return []FileInfo{}, err
	}
	err = utils.Json.Unmarshal(body, &infos)
	if err != nil {
		return []FileInfo{}, err
	}
	return infos, nil
}

func (c *client) Delete(id string) error {
	err := c.checkAuthorization()
	if err != nil {
		return err
	}

	info, err := c.GetInfo(id)
	if err != nil {
		return err
	}
	v := url.Values{}
	v.Set("hashes", info.Hash)
	v.Set("deleteFiles", "false")
	response, err := c.post("/api/v2/torrents/delete", v)
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		return errors.New("failed")
	}
	return nil
}

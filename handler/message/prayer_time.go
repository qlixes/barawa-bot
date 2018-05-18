package message

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type PrayerTimeHandler struct {
	*BaseMessageHandler
	cache *Response
}

type Response struct {
	Status  int
	Message string
	Data    interface{}
}

func (h *PrayerTimeHandler) handleRequestError(err error) string {
	fmt.Println(err)
	return "Gagal ngambil data :("
}

func (h *PrayerTimeHandler) handleParseError(err error) string {
	fmt.Println(err)
	return "Gagal baca data :("
}

func (h *PrayerTimeHandler) setupRequest(client *http.Client, req *http.Request, res *http.Response) {
	req.Header.Set("Host", "bimasislam.kemenag.go.id")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/66.0.3359.181 Safari/537.36")
	if req.Method == "POST" {
		req.Header.Set("Origin", "https://bimasislam.kemenag.go.id")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
		req.Header.Set("Referer", "https://bimasislam.kemenag.go.id/jadwalshalat")
	}
}

func (h *PrayerTimeHandler) handle(message string, params ...interface{}) string {
	if h.cache != nil {
		return h.handleResponse(h.cache)
	}
	now := time.Now()
	// This code is redundant but I'm too lazy to create a function for it :P
	if now.Hour() > 20 {
		now = now.Add(time.Duration(12) * time.Hour)
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: time.Second * 60,
		Jar:     jar,
	}

	req, _ := http.NewRequest("GET", "https://bimasislam.kemenag.go.id/jadwalshalat", nil)
	h.setupRequest(client, req, nil)
	req.Header.Set("Host", "bimasislam.kemenag.go.id")
	res, err := client.Do(req)
	if err != nil {
		return h.handleRequestError(err)
	}
	defer res.Body.Close()

	res, err = client.Do(req)
	if err != nil {
		return h.handleRequestError(err)
	}
	defer res.Body.Close()

	doc, _ := goquery.NewDocumentFromReader(res.Body)
	xParam := ""
	doc.Find("select#search_prov > option").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "JAKARTA") {
			xParam, _ = s.Attr("value")
		}
	})

	u := url.URL{}
	q := u.Query()
	q.Set("x", xParam)
	body := q.Encode()
	req, _ = http.NewRequest("POST", "https://bimasislam.kemenag.go.id/ajax/getKabkoshalat", strings.NewReader(body))
	h.setupRequest(client, req, res)
	res, err = client.Do(req)
	if err != nil {
		return h.handleRequestError(err)
	}
	defer res.Body.Close()

	doc, _ = goquery.NewDocumentFromReader(res.Body)
	yParam, _ := doc.Find("option[data-val='Kota Jakarta']").Attr("value")

	q.Set("y", yParam)
	q.Set("bln", strconv.Itoa(int(now.Month())))
	q.Set("thn", strconv.Itoa(now.Year()))
	body = q.Encode()

	req, _ = http.NewRequest("POST", "https://bimasislam.kemenag.go.id/ajax/getShalatbln", strings.NewReader(body))
	h.setupRequest(client, req, res)
	res, err = client.Do(req)
	if err != nil {
		return h.handleRequestError(err)
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return h.handleParseError(err)
	}
	x := Response{}
	json.Unmarshal(data, &x)
	if x.Status != 1 {
		return h.handleRequestError(errors.New("Server returns data with 0 status"))
	}
	h.cache = &x
	return h.handleResponse(&x)
}

func (h *PrayerTimeHandler) handleResponse(x *Response) string {
	localTime := time.Now().Local()
	hour := localTime.Hour()
	if hour >= 20 {
		localTime = localTime.Add(time.Duration(12) * time.Hour)
	}
	date := localTime.Format("2006-01-02")
	schedules := x.Data.(map[string]interface{})[date]
	response := make([]string, 0)
	keys := []string{
		"tanggal",
		"imsak",
		"subuh",
		"terbit",
		"dhuha",
		"dzuhur",
		"ashar",
		"maghrib",
		"isya",
	}
	for _, k := range keys {
		v := schedules.(map[string]interface{})[k].(string)
		response = append(response, fmt.Sprintf("%s: %s", strings.Title(k), v))
	}
	return strings.Join(response, "\n")
}

func (h *PrayerTimeHandler) test(message string) bool {
	match, _ := regexp.Match(`^kapan waktu (azan|adzan|solat|salat|buka puasa)[?]?$`, []byte(strings.ToLower(message)))
	return match
}
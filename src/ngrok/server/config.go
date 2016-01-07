package server

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/peterbourgon/diskv"
)

type UserConfig struct {
	UserId    string   `json:"userId"`
	AuthId    string   `json:"authId"`
	ForumName string   `json: "forumName"`
	Dns       []string `json:"dns"`
}

type UserInfo struct {
	UserConfig

	transPerDay uint64
	transAll    uint64
}

type DdProvider interface {
	Save(config *UserConfig) error
	LoadAll() error
}

type Db struct {
	diskv *diskv.Diskv
}

type ConfigMgr struct {
	db    *DbProvider
	users map[string]*UserInfo
	dns   map[string]*UserInfo
}

type appHandler struct {
	*ConfigMgr
	h func(*ServerHttpd, http.ResponseWriter, *http.Request) (int, error)
}

func (ah appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status, err := ah.h(ah.ConfigMgr, w, r)
	if err != nil {
		log.Println("HTTP %d: %q\n", status, err)
		switch status {
		case http.StatusNotFound:
			http.NotFound(w, r)
			// And if we wanted a friendlier error page, we can
			// now leverage our context instance - e.g.
			// err := ah.renderTemplate(w, "http_404.tmpl", nil)
		case http.StatusInternalServerError:
			http.Error(w, http.StatusText(status), status)
		default:
			http.Error(w, http.StatusText(status), status)
		}
	}
}

func blockTransform(s string) []string {
	block := 2
	word := 2
	pathSlice := make([]string, block)
	if len(s) < block*word {
		for i := 0; i < block; i++ {
			pathSlice[i] = "__small"
		}
		return pathSlice
	}

	for i := 0; i < block; i++ {
		pathSlice[i] = s[word*i : word*(i+1)]
	}
	return pathSlice
}

func ConfigMain() {
	addr := ":4446"
	path := "/tmp/db-diskv"

	diskv := diskv.New(diskv.Options{
		BasePath:     path,
		Transform:    blockTransform,
		CacheSizeMax: 1024 * 1024, // 1MB
	})
	db := &Db{diskv: diskv}
	mgr := &ConfigMgr{}
	router := mux.NewRouter()
	router.Handle("/adduser", appHandler{mgr, addUser})
	router.Handle("/info")
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./statics/"))))
	http.ListenAndServe(addr, router)
}

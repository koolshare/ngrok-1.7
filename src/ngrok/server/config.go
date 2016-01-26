package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	msg "ngrok/msg"

	"github.com/gorilla/mux"
	"github.com/peterbourgon/diskv"
)

const (
	DbPrefix = "ngrok"
)

type UserConfig struct {
	UserId string   `json:"userId"`
	AuthId string   `json:"authId"`
	Dns    []string `json:"dns"`
}

type UserInfo struct {
	Uc *UserConfig

	TransPerDay int64
	TransAll    int64
}

type DbProvider interface {
	Save(mgr *ConfigMgr, config *UserConfig) error
	LoadAll(mgr *ConfigMgr) error
}

type Db struct {
	diskv *diskv.Diskv
}

type ConfigMgr struct {
	mu    sync.RWMutex
	db    DbProvider
	users map[string]*UserInfo
	dns   map[string]*UserInfo
}

type appHandler struct {
	*ConfigMgr
	h func(*ConfigMgr, http.ResponseWriter, *http.Request) (int, error)
}

func (ah appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status, err := ah.h(ah.ConfigMgr, w, r)
	if err != nil {
		log.Println("HTTP ", status, " : ", err)
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

func (db *Db) Save(mgr *ConfigMgr, uc *UserConfig) error {
	b, _ := json.Marshal(uc)
	db.diskv.Write(DbPrefix+":"+uc.AuthId, b)
	return nil
}

func (db *Db) LoadAll(mgr *ConfigMgr) error {
	keys := db.diskv.KeysPrefix(DbPrefix, nil)
	for k := range keys {
		if uc, err := db.loadFrom(k); err == nil {
			mgr.AddUserConfig(uc)
		} else {
			log.Println("loadFrom db error", err)
		}
	}

	return nil
}

func (db *Db) loadFrom(key string) (*UserConfig, error) {
	var uc UserConfig
	if b, err := db.diskv.Read(key); err == nil {
		if err2 := json.Unmarshal(b, &uc); err2 == nil {
			return &uc, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

//Add new config, but not save to db
func (mgr *ConfigMgr) AddUserConfig(uc *UserConfig) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if _, exists := mgr.users[uc.AuthId]; exists {
		return errors.New("exists")
	}

	for _, dns := range uc.Dns {
		if _, exists := mgr.dns[dns]; exists {
			return errors.New("dns exists")
		}
	}

	ui := &UserInfo{Uc: uc}
	mgr.users[uc.AuthId] = ui
	for _, dns := range uc.Dns {
		mgr.dns[dns] = ui
	}

	return nil
}

func (mgr *ConfigMgr) bindUserInner(id string, user string) (*UserConfig, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	var ui *UserInfo
	var exists bool
	if ui, exists = mgr.users[id]; !exists {
		return nil, errors.New("not exists")
	} else {
		if ui.Uc.UserId != "" {
			return nil, errors.New("already bind")
		}
	}

	if nil == ui.Uc {
		return nil, errors.New("UserConfig not configurate")
	}

	//Set it
	ui.Uc.UserId = user

	return ui.Uc, nil
}

func (mgr *ConfigMgr) BindUser(id string, user string) error {
	if uc, err := mgr.bindUserInner(id, user); err == nil {
		mgr.db.Save(mgr, uc)
		return nil
	} else {
		return err
	}
}

func (mgr *ConfigMgr) ListAll() []string {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	s := make([]string, 0, 256)
	for _, v := range mgr.users {
		b, _ := json.Marshal(v)
		s = append(s, string(b))
	}

	return s
}

func (mgr *ConfigMgr) GetUserInfo(id string) *UserInfo {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if ui, ok := mgr.users[id]; ok {
		return ui
	}

	return nil
}

func (mgr *ConfigMgr) GetByDns(dns string) *UserInfo {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if ui, ok := mgr.dns[dns]; ok {
		return ui
	}

	return nil
}

func (mgr *ConfigMgr) TimeoutAllDays() {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	for _, v := range mgr.users {
		atomic.StoreInt64(&v.TransPerDay, 0)
	}
}

func addUser(mgr *ConfigMgr, w http.ResponseWriter, r *http.Request) (int, error) {
	if opts.pass != r.Header.Get("Auth") {
		return 400, errors.New("not allow")
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 400, err
	}
	log.Println("body:", string(body))

	var uc UserConfig
	if err := json.Unmarshal(body, &uc); err != nil {
		return 400, err
	}

	if err := mgr.AddUserConfig(&uc); err != nil {
		return 400, err
	}

	mgr.db.Save(mgr, &uc)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, "{'code': 'ok'}")
	return 200, nil
}

func showInfo(mgr *ConfigMgr, w http.ResponseWriter, r *http.Request) (int, error) {
	if opts.pass != r.Header.Get("Auth") {
		return 400, errors.New("not allow")
	}

	s := mgr.ListAll()
	for _, ss := range s {
		fmt.Fprint(w, ss+"\n")
	}

	return 200, nil
}

var cMgr *ConfigMgr

func GetMgr() *ConfigMgr {
	return cMgr
}

func GetUserInfo(id string) *UserInfo {
	return cMgr.GetUserInfo(id)
}

func GetByDns(dns string) *UserInfo {
	return cMgr.GetByDns(dns)
}

func (ui *UserInfo) CheckDns(dns string) bool {
	for _, s := range ui.Uc.Dns {
		if s == dns {
			return true
		}
	}

	return false
}

func CheckForLogin(authMsg *msg.Auth) *UserInfo {
	usr := cMgr.GetUserInfo(authMsg.ClientId)
	if usr == nil {
		return nil
	}

	if usr.Uc.UserId != "" && usr.Uc.UserId != authMsg.Password {
		return nil
	}

	if usr.Uc.UserId == "" {
		//bind
		cMgr.BindUser(authMsg.ClientId, authMsg.Password)
	} else {
		day := atomic.LoadInt64(&usr.TransPerDay)
		//bigger than 1G is not allow
		if day > 1024*1024*1024 {
			return nil
		}
	}

	return usr
}

func NewConfigMgr() *ConfigMgr {
	path := "/tmp/db-diskv"

	diskv := diskv.New(diskv.Options{
		BasePath:     path,
		Transform:    blockTransform,
		CacheSizeMax: 1024 * 1024, // 1MB
	})
	db := &Db{diskv: diskv}
	return &ConfigMgr{db: db, users: make(map[string]*UserInfo), dns: make(map[string]*UserInfo)}
}

func ConfigMain() {
	addr := ":4446"
	cMgr = NewConfigMgr()
	cMgr.db.LoadAll(cMgr)

	go func() {
		tick := time.NewTicker(time.Hour * 24)
		for {
			select {
			case <-tick.C:
				cMgr.TimeoutAllDays()
			}
		}
	}()

	router := mux.NewRouter()
	router.Handle("/adduser", appHandler{cMgr, addUser})
	router.Handle("/info", appHandler{cMgr, showInfo})
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./statics/"))))
	http.ListenAndServe(addr, router)
}

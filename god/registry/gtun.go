package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/ICKelin/gtun/common"
	"github.com/ICKelin/gtun/god/config"
	"github.com/ICKelin/gtun/god/models"
	"github.com/ICKelin/gtun/logs"
)

type gtun struct {
	listener string
	tokens   []string
}

func NewGtun(cfg *config.GtunConfig) *gtun {
	return &gtun{
		listener: cfg.Listener,
		tokens:   cfg.Tokens,
	}
}

func (g *gtun) Run() error {
	http.HandleFunc("/gtun/access", g.onGtunAccess)
	return http.ListenAndServe(g.listener, nil)
}

func (g *gtun) onGtunAccess(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logs.Error("read body fail: %v", err)
		return
	}
	defer r.Body.Close()

	regInfo := &common.C2GRegister{}
	err = json.Unmarshal(content, &regInfo)
	if err != nil {
		bytes := common.Response(nil, err)
		w.Write(bytes)
		return
	}

	if g.checkAuth(regInfo) == false {
		bytes := common.Response(nil, errors.New("auth fail"))
		w.Write(bytes)
		return
	}

	gtundInfo, err := models.GetGtundManager().GetAvailableGtund(regInfo.IsWindows)
	if err != nil {
		bytes := common.Response(nil, err)
		w.Write(bytes)
		return
	}

	respObj := &common.G2CRegister{
		ServerAddress: fmt.Sprintf("%s:%d", gtundInfo.PublicIP, gtundInfo.Port),
	}

	bytes := common.Response(respObj, nil)
	w.Write(bytes)
	logs.Info("register from %s", r.RemoteAddr)
}

func (g *gtun) checkAuth(regInfo *common.C2GRegister) bool {
	// just write for...
	for _, token := range g.tokens {
		if token == regInfo.AuthToken {
			return true
		}
	}
	return false
}

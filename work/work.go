package work

import (
	"context"
	"net/http"

	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	ck "github.com/solpipe/kiss-kit/clock"
)

type Work struct {
	ctx    context.Context
	cancel context.CancelFunc
	Rpc    *sgorpc.Client
	Ws     *sgows.Client
}

type Configuration struct {
	RpcUrl  string `json:"rpc"`
	WsUrl   string `json:"ws"`
	Headers map[string]string
}

func Create(
	ctx context.Context,
	config *Configuration,
) (*Work, error) {
	ctxC, cancel := context.WithCancel(ctx)
	var err error
	w := new(Work)
	w.ctx = ctxC
	w.cancel = cancel
	headers := http.Header{}
	if config.Headers != nil {
		headers = mapToHeader(config.Headers)
	}
	w.Rpc = sgorpc.NewWithHeaders(config.RpcUrl, headers)
	w.Ws, err = sgows.ConnectWithOptions(w.ctx, config.WsUrl, &sgows.Options{
		HttpHeader: headers,
	})
	if err != nil {
		cancel()
		return nil, err
	}

	return w, nil
}

func mapToHeader(m map[string]string) http.Header {
	ans := make(http.Header)
	for k, v := range m {
		ans[k] = []string{v}
	}
	return ans
}

func (e1 *Work) Stop() {
	e1.cancel()
}

func (e1 *Work) Clock() (ck.Clock, error) {
	return ck.Create(e1.ctx, e1.Rpc, e1.Ws)
}

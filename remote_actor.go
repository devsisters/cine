package glam

import (
	"fmt"
	"net/rpc"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

type RemoteActor struct {
	pid        Pid
	client     *rpc.Client
	clientOnce sync.Once
}

func (r *RemoteActor) initClient() error {
	var retErr error
	r.clientOnce.Do(func() {
		client, err := rpc.DialHTTP("tcp", r.pid.NodeName)
		if err != nil {
			retErr = fmt.Errorf("RemoteActor dial %v failed", r.pid.NodeName)
		}
		r.client = client
	})
	return retErr
}

func (r *RemoteActor) createRequest(function interface{}, args ...interface{}) RemoteRequest {
	funcName := runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name()
	tokens := strings.Split(funcName, ".")
	funcName = tokens[len(tokens)-1]

	return RemoteRequest{
		Pid:          r.pid,
		FunctionName: funcName,
		Args:         args,
	}
}

func (r *RemoteActor) Call(function interface{}, args ...interface{}) []interface{} {
	r.initClient()
	req := r.createRequest(function, args...)

	var resp RemoteResponse
	call := r.client.Go("DirectorApi.HandleRemoteCall", req, &resp, nil)
	<-call.Done

	return resp.Return
}

func (r *RemoteActor) Cast(out chan<- Response, function interface{}, args ...interface{}) {
	r.initClient()
	req := r.createRequest(function, args...)

	var resp RemoteRequest
	r.client.Go("DirectorApi.HandleRemoteCast", req, &resp, nil)
}

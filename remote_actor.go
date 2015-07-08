package cine

import (
	"fmt"
	"net/rpc"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

type RemoteActor struct {
	actor      Actor
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

func (r *RemoteActor) RemoteCall(function interface{}, args []interface{}) ([]interface{}, error) {
	r.initClient()
	req := r.createRequest(function, args...)

	var resp RemoteResponse
	call := r.client.Go("DirectorApi.HandleRemoteCall", req, &resp, nil)
	<-call.Done
	if call.Error != nil {
		return nil, call.Error
	}

	if resp.Err != nil {
		return nil, resp.Err
	}

	return resp.Return, nil
}

func (r *RemoteActor) RemoteCast(done chan *Call, function interface{}, args []interface{}) {
	r.initClient()
	req := r.createRequest(function, args...)

	var resp RemoteResponse
	r.client.Go("DirectorApi.HandleRemoteCast", req, &resp, nil)
}

func (r *RemoteActor) RemoteStop() *DirectorError {
	r.initClient()
	req := RemoteRequest{
		Pid: r.pid,
	}

	var resp RemoteResponse
	r.client.Go("DirectorApi.HandleRemoteCast", req, &resp, nil)
	if resp.Err != nil {
		return resp.Err
	}
	return nil
}

func (r *RemoteActor) call(function interface{}, args ...interface{}) ([]interface{}, *DirectorError) {
	ret, err := r.actor.call((*RemoteActor).RemoteCall, function, args)
	if err != nil {
		return nil, err
	}
	if ret[1] != nil {
		return nil, ret[1].(*DirectorError)
	} else {
		return ret[0].([]interface{}), nil
	}
}

func (r *RemoteActor) cast(done chan *Call, function interface{}, args ...interface{}) {
	r.actor.cast(done, (*RemoteActor).RemoteCast, done, function, args)
}

func (r *RemoteActor) stop() *DirectorError {
	ret, err := r.actor.call((*RemoteActor).RemoteStop)
	if err != nil {
		return err
	}
	if ret[0] != nil {
		return ret[0].(*DirectorError)
	}
	return nil
}

package cine

import (
	"log"
	"net/rpc"
	"reflect"
	"runtime"
	"strings"
)

type RemoteActor struct {
	pid      Pid
	client   *rpc.Client
	director *Director
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

func (r *RemoteActor) call(function interface{}, args ...interface{}) ([]interface{}, *DirectorError) {
	req := r.createRequest(function, args...)

	var resp RemoteResponse
	call := r.client.Go("DirectorApi.HandleRemoteCall", req, &resp, nil)
	<-call.Done
	if call.Error == rpc.ErrShutdown {
		log.Println("Remote actor rpc.Client shutdown, returning ErrActorNotFound")
		r.director.removeClient(r.pid)
		// TODO(serialx): Add more specific error return
		return nil, ErrActorNotFound
	} else if call.Error != nil {
		log.Printf("Remote actor call failed with: %v, returning ErrActorNotFound\n", call.Error)
		// TODO(serialx): Add more specific error return
		return nil, ErrActorNotFound
	}

	if resp.Err != nil {
		return nil, resp.Err
	}

	return resp.Return, nil
}

func (r *RemoteActor) cast(done chan *ActorCall, function interface{}, args ...interface{}) {
	req := r.createRequest(function, args...)

	var resp RemoteResponse
	r.client.Go("DirectorApi.HandleRemoteCast", req, &resp, nil)
}

func (r *RemoteActor) stop() *DirectorError {
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

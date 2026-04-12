package controller

import "net/http"

func RegisterRoutes(mux *http.ServeMux, ctrl *AnnotationController) {
	mux.HandleFunc("POST /api/annotations", ctrl.handleCreate)
	mux.HandleFunc("GET /api/annotations", ctrl.handleList)
	mux.HandleFunc("GET /api/annotations/{id}", ctrl.handleGet)
	mux.HandleFunc("GET /api/annotations/{id}/image", ctrl.handleGetImage)
	mux.HandleFunc("PUT /api/annotations/{id}", ctrl.handleUpdate)
	mux.HandleFunc("DELETE /api/annotations/{id}", ctrl.handleDelete)
	mux.HandleFunc("POST /api/annotations/{id}/resolve", ctrl.handleResolve)
	mux.HandleFunc("GET /api/settings/channel-mode", ctrl.handleGetChannelMode)
	mux.HandleFunc("PUT /api/settings/channel-mode", ctrl.handleSetChannelMode)
	mux.HandleFunc("POST /api/channel/push", ctrl.handlePush)
}

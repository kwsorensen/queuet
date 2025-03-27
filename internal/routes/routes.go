package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/queuet/internal/handlers"
)

func SetupRoutes(r chi.Router, taskHandler *handlers.TaskHandler) {
	r.Route("/api/v1", func(r chi.Router) {
		// Tasks endpoints
		r.Route("/tasks", func(r chi.Router) {
			r.Get("/", taskHandler.ListTasks)
			r.Post("/", taskHandler.CreateTask)
			r.Get("/{id}", taskHandler.GetTask)
			r.Put("/{id}", taskHandler.UpdateTask)
			r.Delete("/{id}", taskHandler.DeleteTask)
		})
	})
}

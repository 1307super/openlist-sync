package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *Handlers, staticFS fs.FS) {
	api := r.Group("/api")
	{
		api.GET("/settings", h.GetSettings)
		api.PUT("/settings", h.UpdateSettings)
		api.POST("/settings/test", h.TestSettings)

		api.GET("/tasks", h.ListTasks)
		api.POST("/tasks", h.CreateTask)
		api.GET("/tasks/:id", h.GetTask)
		api.PUT("/tasks/:id", h.UpdateTask)
		api.DELETE("/tasks/:id", h.DeleteTask)
		api.POST("/tasks/:id/start", h.StartTask)
		api.POST("/tasks/:id/stop", h.StopTask)
		api.POST("/tasks/:id/trigger", h.TriggerTask)
		api.GET("/tasks/:id/logs", h.GetTaskLogs)
		api.GET("/tasks/:id/jobs", h.GetTaskJobs)
		api.GET("/tasks/:id/progress", h.SyncProgress)

		api.POST("/browse/list", h.BrowseList)
		api.POST("/browse/dirs", h.BrowseDirs)

		api.GET("/sync/status", h.SyncStatus)
	}

	if staticFS != nil {
		serveSPA(r, staticFS)
	}
}

func serveSPA(r *gin.Engine, fsys fs.FS) {
	fileServer := http.FileServer(http.FS(fsys))

	r.GET("/assets/*filepath", func(c *gin.Context) {
		c.Request.URL.Path = "/assets" + c.Param("filepath")
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	indexHTML, _ := fs.ReadFile(fsys, "index.html")

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
			return
		}

		if path != "/" {
			f, err := fsys.Open(strings.TrimPrefix(path, "/"))
			if err == nil {
				f.Close()
				c.Request.URL.Path = path
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})
}

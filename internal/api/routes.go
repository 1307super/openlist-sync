package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *Handlers, authSecret string, staticFS fs.FS) {
	api := r.Group("/api")

	api.POST("/auth/login", LoginHandler(h.db, authSecret))

	protected := api.Group("")
	protected.Use(AuthMiddleware(h.db, authSecret))
	{
		protected.GET("/settings", h.GetSettings)
		protected.PUT("/settings", h.UpdateSettings)
		protected.POST("/settings/test", h.TestSettings)

		protected.GET("/tasks", h.ListTasks)
		protected.POST("/tasks", h.CreateTask)
		protected.GET("/tasks/:id", h.GetTask)
		protected.PUT("/tasks/:id", h.UpdateTask)
		protected.DELETE("/tasks/:id", h.DeleteTask)
		protected.POST("/tasks/:id/start", h.StartTask)
		protected.POST("/tasks/:id/stop", h.StopTask)
		protected.POST("/tasks/:id/trigger", h.TriggerTask)
		protected.GET("/tasks/:id/logs", h.GetTaskLogs)
		protected.GET("/tasks/:id/jobs", h.GetTaskJobs)
		protected.DELETE("/tasks/:id/jobs/:jobId", h.DeleteTaskJob)
		protected.GET("/tasks/:id/progress", h.SyncProgress)

		protected.POST("/browse/list", h.BrowseList)
		protected.POST("/browse/dirs", h.BrowseDirs)

		protected.GET("/sync/status", h.SyncStatus)
		protected.DELETE("/logs", h.ClearLogs)

		protected.GET("/openlist/copy-tasks", h.OpenListCopyTasks)

		protected.GET("/monitor/config", h.GetMonitorConfig)
		protected.PUT("/monitor/config", h.UpdateMonitorConfig)
		protected.GET("/monitor/dirs", h.ListMonitorDirs)
		protected.POST("/monitor/dirs", h.AddMonitorDir)
		protected.DELETE("/monitor/dirs/:id", h.DeleteMonitorDir)
		protected.POST("/monitor/trigger", h.TriggerMonitor)
		protected.GET("/monitor/status", h.MonitorStatus)
		protected.PUT("/monitor/scan-time", h.UpdateMonitorScanTime)
		protected.GET("/monitor/logs", h.GetMonitorLogs)
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

package main

import (
	"compress/gzip"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

func registerDataRoutes(app core.App, r *router.Router[*core.RequestEvent], requireSession, requireConsent func(*core.RequestEvent) error) {
	r.GET("/test", func(c *core.RequestEvent) error {
		return c.String(http.StatusOK, "Research steps template API is running")
	})

	r.POST("/data", func(c *core.RequestEvent) error {
		reqBody := struct {
			UserID     string     `json:"userId"`
			ChunkIndex int        `json:"chunkIndex"`
			Data       []DataItem `json:"data"`
		}{}
		if err := c.BindBody(&reqBody); err != nil {
			return httpError(http.StatusBadRequest, "Invalid request body")
		}

		userID, err := authorizedUser(c, reqBody.UserID)
		if err != nil {
			return err
		}

		if reqBody.ChunkIndex < 0 {
			return httpError(http.StatusBadRequest, "Invalid chunkIndex")
		}
		if len(reqBody.Data) == 0 {
			return httpError(http.StatusBadRequest, "No data points provided")
		}

		dataHash, err := hashDataItems(reqBody.Data)
		if err != nil {
			log.Println("Error hashing upload:", err)
			return httpError(http.StatusInternalServerError, "Failed to process upload")
		}

		now := time.Now().UTC()
		lock := getUploadLock(userID)
		lock.Lock()
		defer lock.Unlock()

		state, err := loadUploadState(userID)
		if err != nil {
			return httpError(http.StatusInternalServerError, "Failed to read upload state")
		}

		switch {
		case reqBody.ChunkIndex == 0:
			if state == nil || isUploadStateExpired(state, now) {
				state, err = createNewUploadState(userID, now)
				if err != nil {
					return httpError(http.StatusInternalServerError, "Failed to start upload")
				}
			} else if state.ExpectedChunk > 0 {
				duplicateFirstChunk, err := isDuplicateChunk(userID, state, 0, dataHash)
				if err != nil {
					return httpError(http.StatusInternalServerError, "Failed to process upload")
				}
				if !duplicateFirstChunk {
					state, err = createNewUploadState(userID, now)
					if err != nil {
						return httpError(http.StatusInternalServerError, "Failed to restart upload")
					}
				}
			}
		case state == nil || isUploadStateExpired(state, now):
			return httpError(http.StatusConflict, "No active upload session. Restart from chunk 0.")
		}

		if reqBody.ChunkIndex > state.ExpectedChunk {
			return httpError(
				http.StatusConflict,
				fmt.Sprintf("Out-of-order chunk. Expected chunk %d.", state.ExpectedChunk),
			)
		}

		if reqBody.ChunkIndex < state.ExpectedChunk {
			isDuplicate, err := isDuplicateChunk(userID, state, reqBody.ChunkIndex, dataHash)
			if err != nil {
				return httpError(http.StatusInternalServerError, "Failed to process upload")
			}
			if !isDuplicate {
				return httpError(http.StatusConflict, "Chunk already received with different content.")
			}

			filePath := chunkFilePath(userID, state.SessionID, reqBody.ChunkIndex)
			return c.JSON(http.StatusOK, map[string]any{
				"message":    "Chunk already received",
				"filePath":   filePath,
				"sessionId":  state.SessionID,
				"chunkIndex": reqBody.ChunkIndex,
			})
		}

		filePath := chunkFilePath(userID, state.SessionID, reqBody.ChunkIndex)
		if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
			return httpError(http.StatusInternalServerError, "Failed to prepare storage directory")
		}

		if err := writeCompressedFile(filePath, reqBody.Data); err != nil {
			return httpError(http.StatusInternalServerError, "Failed to persist upload chunk")
		}
		if err := saveChunkHash(userID, state, reqBody.ChunkIndex, dataHash); err != nil {
			return httpError(http.StatusInternalServerError, "Failed to persist chunk metadata")
		}

		state.ExpectedChunk++
		state.UpdatedAt = now
		if err := saveUploadState(userID, state); err != nil {
			return httpError(http.StatusInternalServerError, "Failed to persist upload state")
		}

		dataFrom, dataTo, err := getEarliestAndLatestDates(reqBody.Data)
		if err != nil {
			return httpError(http.StatusInternalServerError, "Failed to read upload date coverage")
		}

		collection, err := app.FindCollectionByNameOrId("dataUploads")
		if err != nil {
			return httpError(http.StatusInternalServerError, "Failed to find uploads collection")
		}

		user, err := app.FindRecordById("users", userID)
		if err != nil {
			return httpError(http.StatusNotFound, "Participant not found")
		}

		record := core.NewRecord(collection)
		record.Set("user", user.Id)
		record.Set("filePath", filePath)
		record.Set("timestamp", now)
		record.Set("dataFrom", dataFrom)
		record.Set("dataTo", dataTo)
		if err := app.Save(record); err != nil {
			return httpError(http.StatusInternalServerError, "Failed to save upload metadata")
		}

		return c.JSON(http.StatusOK, map[string]any{
			"message":    "Data saved successfully",
			"filePath":   filePath,
			"sessionId":  state.SessionID,
			"chunkIndex": reqBody.ChunkIndex,
		})
	}).Bind(apis.BodyLimit(200*1024*1024)).BindFunc(requireSession, requireConsent, decompressUpload)

	r.GET("/data/{userId}", requireAPIKey(func(c *core.RequestEvent) error {
		userID, err := sanitizeUserID(c.Request.PathValue("userId"))
		if err != nil {
			return httpError(http.StatusBadRequest, "Invalid userId")
		}

		lock := getUploadLock(userID)
		lock.Lock()
		defer lock.Unlock()

		sessionDirs, err := listSessionDirectories(userID)
		if err != nil {
			return httpError(http.StatusInternalServerError, "Failed to list upload sessions")
		}

		for index := len(sessionDirs) - 1; index >= 0; index-- {
			allData, err := readDataFromSession(userID, sessionDirs[index])
			if err != nil {
				return httpError(http.StatusInternalServerError, "Failed to read upload session")
			}
			if len(allData) > 0 {
				return c.JSON(http.StatusOK, allData)
			}
		}

		return c.JSON(http.StatusOK, []DataItem{})
	}))

}

func requireAPIKey(next func(*core.RequestEvent) error) func(*core.RequestEvent) error {
	return func(c *core.RequestEvent) error {
		apiKey := os.Getenv("API_KEY")
		if apiKey == "" {
			log.Println("WARNING: API_KEY environment variable is not set")
			return httpError(http.StatusInternalServerError, "Server misconfigured")
		}

		providedKey := c.Request.Header.Get("X-API-Key")
		if providedKey == "" || subtle.ConstantTimeCompare([]byte(providedKey), []byte(apiKey)) != 1 {
			return httpError(http.StatusUnauthorized, "Invalid or missing API key")
		}

		return next(c)
	}
}

func httpError(status int, message string) error { return apis.NewApiError(status, message, nil) }

// Every upload identifies its participant and must match the authenticated user.
func authorizedUser(e *core.RequestEvent, supplied string) (string, error) {
	if e.Auth == nil {
		return "", httpError(401, "Participant authentication required")
	}
	id, err := sanitizeUserID(e.Auth.Id)
	if err != nil || supplied != id {
		return "", httpError(403, "Participant does not match authenticated session")
	}
	return id, nil
}

func decompressUpload(e *core.RequestEvent) error {
	const maxSize = 200 * 1024 * 1024
	e.Request.Body = http.MaxBytesReader(e.Response, e.Request.Body, maxSize)
	encoding := e.Request.Header.Get("Content-Encoding")
	if encoding != "" && encoding != "gzip" {
		return httpError(415, "Unsupported content encoding")
	}
	if encoding == "gzip" {
		reader, err := gzip.NewReader(e.Request.Body)
		if err != nil {
			return httpError(400, "Invalid gzip body")
		}
		defer reader.Close()
		e.Request.Body = http.MaxBytesReader(e.Response, reader, maxSize)
		e.Request.Header.Del("Content-Encoding")
		e.Request.ContentLength = -1
	}
	return e.Next()
}

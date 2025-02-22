package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to find video with ID: "+videoID.String(), err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "This video doesn't belong to you", err)
		return
	}

	contentType := header.Header.Get("Content-Type")

	mediatype, _, _ := mime.ParseMediaType(contentType)

	ext, ok := allowedImageTypes[mediatype]
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Unable to use image of type "+contentType, err)
		return
	}

	filename := fmt.Sprintf("%s.%s", videoID, ext)
	thumbnailPath := filepath.Join(cfg.assetsRoot, filename)

	thumbnail, err := os.Create(thumbnailPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to store thumbnail: "+filename, err)
		return
	}

	_, err = io.Copy(thumbnail, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to save thumbnail: "+filename, err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, filename)

	video.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video with ID: "+videoID.String(), err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {

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

	fmt.Println("uploading video", videoID, "by user", userID)

	const maxMemory = 1 << 30
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("video")
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

	ext, ok := allowedVideoTypes[mediatype]
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Unable to use video of type "+contentType, err)
		return
	}

	filename := fmt.Sprintf("%s.%s", videoID, ext)

	videoFile, err := os.CreateTemp("", filename)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to store video: "+filename, err)
		return
	}

	defer os.Remove(videoFile.Name())
	defer videoFile.Close()

	_, err = io.Copy(videoFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to save video: "+filename, err)
		return
	}

	videoFile.Seek(0, io.SeekStart)

	rn := make([]byte, 32)
	rand.Read(rn)

	randname := base64.RawURLEncoding.EncodeToString(rn)
	randfilename := fmt.Sprintf("%s.%s", randname, ext)

	_, err = cfg.s3Client.PutObject(
		r.Context(),
		&s3.PutObjectInput{
			Bucket:      &cfg.s3Bucket,
			Key:         &randfilename,
			Body:        videoFile,
			ContentType: &contentType,
		},
	)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to upload video with ID: "+videoID.String(), err)
		return
	}

	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, randfilename)
	video.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video with ID: "+videoID.String(), err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)

}

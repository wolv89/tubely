package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {

	if video.VideoURL == nil {
		return video, nil
	}

	parts := strings.Split(*video.VideoURL, ",")

	if len(parts) != 2 {
		return video, errors.New("malformed video url")
	}

	bucket, key := parts[0], parts[1]

	expiry, _ := time.ParseDuration("1h")

	purl, err := generatePresignedURL(cfg.s3Client, bucket, key, expiry)
	if err != nil {
		return video, err
	}

	video.VideoURL = &purl
	return video, nil

}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {

	pc := s3.NewPresignClient(s3Client)

	preq, err := pc.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}, s3.WithPresignExpires(expireTime))

	if err != nil {
		return "", err
	}

	return preq.URL, nil

}

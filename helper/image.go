package helper

import (
	"context"
	"fmt"
	"image"
	"log"
	"mime/multipart"
	"os"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
)

const (
	targetWidth  = 800
	targetHeight = 800
)

var cld *cloudinary.Cloudinary

func InitCloudinary() error {
	cloudinaryURL := os.Getenv("CLOUDINARY_URL")
	if cloudinaryURL == "" {
		return fmt.Errorf("CLOUDINARY_URL not set in environment")
	}

	var err error
	cld, err = cloudinary.NewFromURL(cloudinaryURL)
	if err != nil {
		return fmt.Errorf("failed to initialize Cloudinary: %v", err)
	}
	log.Printf("[INFO] Cloudinary initialized successfully")
	return nil
}

func ProcessImage(c *gin.Context, file multipart.File, header *multipart.FileHeader) (string, error) {
	log.Printf("[INFO] Processing image: %s", header.Filename)

	if seeker, ok := file.(interface {
		Seek(int64, int) (int64, error)
	}); ok {
		pos, err := seeker.Seek(0, 0)
		if err != nil {
			log.Printf("[ERROR] Failed to seek to start of file %s: %v", header.Filename, err)
			return "", fmt.Errorf("failed to seek to start of file: %v", err)
		}
		log.Printf("[INFO] Seek successful, position: %d", pos)
	} else {
		log.Printf("[ERROR] File %s does not support seeking", header.Filename)
		return "", fmt.Errorf("file does not support seeking")
	}

	img, format, err := image.Decode(file)
	if err != nil {
		log.Printf("[ERROR] Failed to decode image %s (format: %s): %v", header.Filename, format, err)
		return "", fmt.Errorf("failed to decode image: %v", err)
	}
	log.Printf("[INFO] Image decoded successfully, format: %s", format)

	bounds := img.Bounds()
	if bounds.Dx() != targetWidth || bounds.Dy() != targetHeight {
		resized := imaging.Resize(img, targetWidth, targetHeight, imaging.Lanczos)
		log.Printf("[INFO] Image resized to %d x %d", targetWidth, targetHeight)

		tempFile := fmt.Sprintf("temp_%d_%s.png", time.Now().UnixNano(), header.Filename)
		err = imaging.Save(resized, tempFile)
		if err != nil {
			log.Printf("[ERROR] Failed to save temp image %s: %v", tempFile, err)
			return "", fmt.Errorf("failed to save temp image: %v", err)
		}
		log.Printf("[INFO] Temporary file saved: %s", tempFile)
		defer os.Remove(tempFile)

		if cld == nil {
			log.Printf("[ERROR] Cloudinary client not initialized")
			return "", fmt.Errorf("Cloudinary client not initialized")
		}

		ctx := context.Background()
		uploadResult, err := cld.Upload.Upload(ctx, tempFile, uploader.UploadParams{
			Folder: "ecommerce/products",
		})
		if err != nil {
			log.Printf("[ERROR] Failed to upload %s to Cloudinary: %v", tempFile, err)
			return "", fmt.Errorf("failed to upload to Cloudinary: %v", err)
		}
		log.Printf("[INFO] Image uploaded to Cloudinary, URL: %s", uploadResult.SecureURL)
		return uploadResult.SecureURL, nil
	}

	tempFile := fmt.Sprintf("temp_%d_%s", time.Now().UnixNano(), header.Filename)
	err = imaging.Save(img, tempFile)
	if err != nil {
		log.Printf("[ERROR] Failed to save temp image %s: %v", tempFile, err)
		return "", fmt.Errorf("failed to save temp image: %v", err)
	}
	defer os.Remove(tempFile)

	ctx := context.Background()
	uploadResult, err := cld.Upload.Upload(ctx, tempFile, uploader.UploadParams{
		Folder: "ecommerce/products",
	})
	if err != nil {
		log.Printf("[ERROR] Failed to upload %s to Cloudinary: %v", tempFile, err)
		return "", fmt.Errorf("failed to upload to Cloudinary: %v", err)
	}
	log.Printf("[INFO] Image uploaded to Cloudinary, URL: %s", uploadResult.SecureURL)
	return uploadResult.SecureURL, nil
}


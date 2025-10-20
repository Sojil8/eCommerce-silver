package helper

import (
	"context"
	"fmt"
	"image"
	"mime/multipart"
	"os"
	"time"

	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"go.uber.org/zap"
)

const (
	targetWidth  = 800
	targetHeight = 800
)

var cld *cloudinary.Cloudinary

func InitCloudinary() error {
	pkg.Log.Debug("Initializing Cloudinary client")

	cloudinaryURL := os.Getenv("CLOUDINARY_URL")
	if cloudinaryURL == "" {
		pkg.Log.Error("CLOUDINARY_URL not set in environment")
		return fmt.Errorf("CLOUDINARY_URL not set in environment")
	}

	var err error
	cld, err = cloudinary.NewFromURL(cloudinaryURL)
	if err != nil {
		pkg.Log.Error("Failed to initialize Cloudinary",
			zap.Error(err))
		return fmt.Errorf("failed to initialize Cloudinary: %w", err)
	}

	pkg.Log.Info("Cloudinary initialized successfully")
	return nil
}

func ProcessImage(c *gin.Context, file multipart.File, header *multipart.FileHeader) (string, error) {
	pkg.Log.Debug("Processing image",
		zap.String("filename", header.Filename),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method))

	if seeker, ok := file.(interface {
		Seek(int64, int) (int64, error)
	}); ok {
		pos, err := seeker.Seek(0, 0)
		if err != nil {
			pkg.Log.Error("Failed to seek to start of file",
				zap.String("filename", header.Filename),
				zap.Error(err))
			return "", fmt.Errorf("failed to seek to start of file: %w", err)
		}
		pkg.Log.Debug("Seek successful",
			zap.String("filename", header.Filename),
			zap.Int64("position", pos))
	} else {
		pkg.Log.Error("File does not support seeking",
			zap.String("filename", header.Filename))
		return "", fmt.Errorf("file does not support seeking")
	}

	img, format, err := image.Decode(file)
	if err != nil {
		pkg.Log.Error("Failed to decode image",
			zap.String("filename", header.Filename),
			zap.String("format", format),
			zap.Error(err))
		return "", fmt.Errorf("failed to decode image: %w", err)
	}
	pkg.Log.Info("Image decoded successfully",
		zap.String("filename", header.Filename),
		zap.String("format", format))

	bounds := img.Bounds()
	if bounds.Dx() != targetWidth || bounds.Dy() != targetHeight {
		resized := imaging.Resize(img, targetWidth, targetHeight, imaging.Lanczos)
		pkg.Log.Info("Image resized",
			zap.String("filename", header.Filename),
			zap.Int("width", targetWidth),
			zap.Int("height", targetHeight))

		tempFile := fmt.Sprintf("temp_%d_%s.png", time.Now().UnixNano(), header.Filename)
		err = imaging.Save(resized, tempFile)
		if err != nil {
			pkg.Log.Error("Failed to save temporary image",
				zap.String("tempFile", tempFile),
				zap.Error(err))
			return "", fmt.Errorf("failed to save temp image: %w", err)
		}
		pkg.Log.Debug("Temporary file saved",
			zap.String("tempFile", tempFile))
		defer os.Remove(tempFile)

		if cld == nil {
			pkg.Log.Error("Cloudinary client not initialized",
				zap.String("filename", header.Filename))
			return "", fmt.Errorf("cloudinary client not initialized")
		}

		ctx := context.Background()
		uploadResult, err := cld.Upload.Upload(ctx, tempFile, uploader.UploadParams{
			Folder: "ecommerce/products",
		})
		if err != nil {
			pkg.Log.Error("Failed to upload image to Cloudinary",
				zap.String("tempFile", tempFile),
				zap.Error(err))
			return "", fmt.Errorf("failed to upload to Cloudinary: %w", err)
		}
		pkg.Log.Info("Image uploaded to Cloudinary",
			zap.String("filename", header.Filename),
			zap.String("secureURL", uploadResult.SecureURL))
		return uploadResult.SecureURL, nil
	}

	tempFile := fmt.Sprintf("temp_%d_%s", time.Now().UnixNano(), header.Filename)
	err = imaging.Save(img, tempFile)
	if err != nil {
		pkg.Log.Error("Failed to save temporary image",
			zap.String("tempFile", tempFile),
			zap.Error(err))
		return "", fmt.Errorf("failed to save temp image: %w", err)
	}
	pkg.Log.Debug("Temporary file saved",
		zap.String("tempFile", tempFile))
	defer os.Remove(tempFile)

	if cld == nil {
		pkg.Log.Error("Cloudinary client not initialized",
			zap.String("filename", header.Filename))
		return "", fmt.Errorf("cloudinary client not initialized")
	}

	ctx := context.Background()
	uploadResult, err := cld.Upload.Upload(ctx, tempFile, uploader.UploadParams{
		Folder: "ecommerce/products",
	})
	if err != nil {
		pkg.Log.Error("Failed to upload image to Cloudinary",
			zap.String("tempFile", tempFile),
			zap.Error(err))
		return "", fmt.Errorf("failed to upload to Cloudinary: %w", err)
	}
	pkg.Log.Info("Image uploaded to Cloudinary",
		zap.String("filename", header.Filename),
		zap.String("secureURL", uploadResult.SecureURL))
	return uploadResult.SecureURL, nil
}
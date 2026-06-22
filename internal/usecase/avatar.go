package usecase

import "fmt"

// maxAvatarBytes caps avatar uploads at 5 MB.
const maxAvatarBytes = 5 * 1024 * 1024

// avatarExtFromContentType maps a detected MIME type to a file extension.
// Only JPEG, PNG and WebP are accepted.
func avatarExtFromContentType(ct string) (string, error) {
	switch ct {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/webp":
		return ".webp", nil
	}
	return "", fmt.Errorf("unsupported content type: %s", ct)
}

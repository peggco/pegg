package llm

import "encoding/base64"

type AttachmentType string

const (
	AttachmentTypeImage AttachmentType = "image"
	AttachmentTypeAudio AttachmentType = "audio"
	AttachmentTypeVideo AttachmentType = "video"
	AttachmentTypeFile  AttachmentType = "file"
)

type Attachment struct {
	Type      AttachmentType `json:"type"`
	MediaType string         `json:"media_type,omitempty"`
	Data      string         `json:"data,omitempty"`
	URL       string         `json:"url,omitempty"`
	FileName  string         `json:"file_name,omitempty"`
	Path      string         `json:"path,omitempty"`
}

func NewAttachmentFromBase64(attType AttachmentType, mediaType, data string) Attachment {
	return Attachment{
		Type:      attType,
		MediaType: mediaType,
		Data:      data,
	}
}

func NewAttachmentFromURL(attType AttachmentType, mediaType, url string) Attachment {
	return Attachment{
		Type:      attType,
		MediaType: mediaType,
		URL:       url,
	}
}

func NewImageAttachmentFromBase64(mediaType, data string) Attachment {
	return NewAttachmentFromBase64(AttachmentTypeImage, mediaType, data)
}

func NewImageAttachmentFromURL(url string) Attachment {
	return NewAttachmentFromURL(AttachmentTypeImage, "", url)
}

func NewAudioAttachmentFromBase64(mediaType, data string) Attachment {
	return NewAttachmentFromBase64(AttachmentTypeAudio, mediaType, data)
}

func NewAudioAttachmentFromURL(url string) Attachment {
	return NewAttachmentFromURL(AttachmentTypeAudio, "", url)
}

func NewVideoAttachmentFromBase64(mediaType, data string) Attachment {
	return NewAttachmentFromBase64(AttachmentTypeVideo, mediaType, data)
}

func NewVideoAttachmentFromURL(url string) Attachment {
	return NewAttachmentFromURL(AttachmentTypeVideo, "", url)
}

func NewFileAttachmentFromBase64(mediaType, data, fileName string) Attachment {
	att := NewAttachmentFromBase64(AttachmentTypeFile, mediaType, data)
	att.FileName = fileName
	return att
}

func NewFileAttachmentFromURL(url, fileName string) Attachment {
	att := NewAttachmentFromURL(AttachmentTypeFile, "", url)
	att.FileName = fileName
	return att
}

func EncodeFileToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

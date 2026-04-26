package messages

import (
	"context"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func SendMessage(client *mautrix.Client, roomId id.RoomID, message string) error {
	slog.Debug("SendMessage", "msg", message, "roomId", roomId)
	ctx := context.Background()
	content := event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    message,
	}
	_, err := client.SendMessageEvent(ctx, roomId, event.EventMessage, content)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func SendMediaMessage(client *mautrix.Client, roomId id.RoomID, filePath, message string) error {
	slog.Debug("SendMediaMessage", "file", filePath, "roomId", roomId)
	ctx := context.Background()

	f, err := os.Open(filePath)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	fileName := filepath.Base(filePath)
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	slog.Debug("uploading media", "file", filePath, "size", stat.Size(), "mime", mimeType)
	resp, err := client.UploadMedia(ctx, mautrix.ReqUploadMedia{
		Content:       f,
		FileName:      fileName,
		ContentType:   mimeType,
		ContentLength: stat.Size(),
	})
	if err != nil {
		slog.Error("upload failed", "err", err)
		debug.PrintStack()
		return err
	}

	content := buildMediaContent(resp.ContentURI, fileName, mimeType, message)

	// 5. Send the event
	_, err = client.SendMessageEvent(ctx, roomId, event.EventMessage, content)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	return nil
}

func buildMediaContent(uri id.ContentURI, fileName, mimeType, message string) event.MessageEventContent {
	var msgType event.MessageType
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		msgType = event.MsgImage
	case strings.HasPrefix(mimeType, "audio/"):
		msgType = event.MsgAudio
	case strings.HasPrefix(mimeType, "video/"):
		msgType = event.MsgVideo
	default:
		msgType = event.MsgFile
	}
	return event.MessageEventContent{
		MsgType:  msgType,
		Body:     message,
		FileName: fileName,
		URL:      uri.CUString(),
		Info: &event.FileInfo{
			MimeType: mimeType,
		},
	}
}

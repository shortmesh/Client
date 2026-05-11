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

func SendMessage(
	client *mautrix.Client,
	roomId id.RoomID,
	message,
	replyId string,
) (*id.EventID, error) {
	slog.Debug("SendMessage", "msg", message, "roomId", roomId)
	ctx := context.Background()
	content := event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    message,
	}
	if replyId != "" {
		content.RelatesTo = &event.RelatesTo{
			InReplyTo: &event.InReplyTo{
				EventID: id.EventID(replyId),
			},
		}
	}

	evt, err := client.SendMessageEvent(ctx, roomId, event.EventMessage, content)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return &evt.EventID, nil
}

func SendMediaMessage(
	client *mautrix.Client,
	roomId id.RoomID,
	filePath,
	message,
	replyId string,
) (*id.EventID, error) {
	slog.Debug("SendMediaMessage", "file", filePath, "roomId", roomId)
	ctx := context.Background()

	f, err := os.Open(filePath)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
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
		return nil, err
	}

	content := buildMediaContent(resp.ContentURI, fileName, mimeType, message, replyId)

	evt, err := client.SendMessageEvent(ctx, roomId, event.EventMessage, content)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}
	return &evt.EventID, nil
}

func buildMediaContent(uri id.ContentURI, fileName, mimeType, message, replyId string) event.MessageEventContent {
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
	evt := event.MessageEventContent{
		MsgType:  msgType,
		Body:     message,
		FileName: fileName,
		URL:      uri.CUString(),
		Info: &event.FileInfo{
			MimeType: mimeType,
		},
	}

	if replyId != "" {
		evt.RelatesTo = &event.RelatesTo{
			InReplyTo: &event.InReplyTo{
				EventID: id.EventID(replyId),
			},
		}
	}

	return evt
}

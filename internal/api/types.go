// SPDX-License-Identifier: GPL-3.0-or-later

package api

import "time"

type Board struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OwnerRaw      Owner   `json:"owner"`
	Color         string  `json:"color"`
	Archived      bool    `json:"archived"`
	Labels        []Label `json:"labels"`
	Stacks        []Stack `json:"stacks,omitempty"`
	Permissions   Perms   `json:"permissions"`
	LastModified  int64   `json:"lastModified"`
	DeletedAt     int64   `json:"deletedAt"`
	ETag          string  `json:"ETag,omitempty"`
}

type Owner struct {
	PrimaryKey string `json:"primaryKey"`
	UID        string `json:"uid"`
	DisplayName string `json:"displayname"`
	Type       int    `json:"type"`
}

type Perms struct {
	Read   bool `json:"PERMISSION_READ"`
	Edit   bool `json:"PERMISSION_EDIT"`
	Manage bool `json:"PERMISSION_MANAGE"`
	Share  bool `json:"PERMISSION_SHARE"`
}

type Stack struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	BoardID      int    `json:"boardId"`
	Order        int    `json:"order"`
	LastModified int64  `json:"lastModified"`
	DeletedAt    int64  `json:"deletedAt"`
	Cards        []Card `json:"cards,omitempty"`
	ETag         string `json:"ETag,omitempty"`
}

type Card struct {
	ID               int          `json:"id"`
	Title            string       `json:"title"`
	Description      string       `json:"description"`
	StackID          int          `json:"stackId"`
	Type             string       `json:"type"`
	LastModified     int64        `json:"lastModified"`
	CreatedAt        int64        `json:"createdAt"`
	Labels           []Label      `json:"labels"`
	AssignedUsers    []Assignment `json:"assignedUsers"`
	Attachments      []Attachment `json:"attachments,omitempty"`
	AttachmentCount  int          `json:"attachmentCount"`
	Owner            Owner        `json:"owner"`
	Order            int          `json:"order"`
	Archived         bool         `json:"archived"`
	DueDate          *string      `json:"duedate"`
	Done             *string      `json:"done"`
	CommentsUnread   int          `json:"commentsUnread"`
	CommentsCount    int          `json:"commentsCount"`
	OverDue          int          `json:"overdue"`
	ETag             string       `json:"ETag,omitempty"`
}

type Label struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	Color        string `json:"color"`
	BoardID      int    `json:"boardId"`
	CardID       int    `json:"cardId,omitempty"`
	LastModified int64  `json:"lastModified"`
	ETag         string `json:"ETag,omitempty"`
}

type Assignment struct {
	ID          int    `json:"id"`
	ParticipantOwner `json:"participant"`
	CardID      int    `json:"cardId"`
	Type        int    `json:"type"`
}

type ParticipantOwner struct {
	PrimaryKey  string `json:"primaryKey"`
	UID         string `json:"uid"`
	DisplayName string `json:"displayname"`
}

type Attachment struct {
	ID           int    `json:"id"`
	CardID       int    `json:"cardId"`
	Type         string `json:"type"`
	Data         string `json:"data"`
	LastModified int64  `json:"lastModified"`
	CreatedAt    int64  `json:"createdAt"`
	CreatedBy    string `json:"createdBy"`
	DeletedAt    int64  `json:"deletedAt"`
	Extended     map[string]any `json:"extendedData,omitempty"`
}

type Comment struct {
	ID           int       `json:"id"`
	ObjectID     int       `json:"objectId"`
	Message      string    `json:"message"`
	ActorID      string    `json:"actorId"`
	ActorDisplay string    `json:"actorDisplayName"`
	ActorType    string    `json:"actorType"`
	CreationDT   time.Time `json:"creationDateTime"`
	Mentions     []any     `json:"mentions"`
	ReplyTo      *Comment  `json:"replyTo,omitempty"`
}

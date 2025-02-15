// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dbr

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// PostVisibleT 可访问类型，可见性: 0私密 10充电可见 20订阅可见 30保留 40保留 50好友可见 60关注可见 70保留 80保留 90公开',
type PostVisibleT uint8

const (
	PostVisitPublic    PostVisibleT = 90
	PostVisitPrivate   PostVisibleT = 0
	PostVisitFriend    PostVisibleT = 50
	PostVisitFollowing PostVisibleT = 60
)

type PostByMedia = Post

type PostByComment = Post

// Post 表示帖子数据结构。
type Post struct {
	*Model
	UserID          int64        `json:"user_id"`           // 用户ID
	CommentCount    int64        `json:"comment_count"`     // 评论数
	CollectionCount int64        `json:"collection_count"`  // 收藏数
	ShareCount      int64        `json:"share_count"`       // 分享数
	UpvoteCount     int64        `json:"upvote_count"`      // 点赞数
	Visibility      PostVisibleT `json:"visibility"`        // 可见性
	IsTop           int          `json:"is_top"`            // 是否置顶
	IsEssence       int          `json:"is_essence"`        // 是否精华
	IsLock          int          `json:"is_lock"`           // 是否锁定
	LatestRepliedOn int64        `json:"latest_replied_on"` // 最新回复时间
	Tags            string       `json:"tags"`              // 标签
	IP              string       `json:"ip"`                // IP地址
	IPLoc           string       `json:"ip_loc"`            // IP地理位置
}

// PostFormated 表示格式化后的帖子结构。
type PostFormated struct {
	ID              int64                  `json:"id"`                // 帖子ID
	UserID          int64                  `json:"user_id"`           // 用户ID
	User            *UserFormated          `json:"user"`              // 用户信息
	Contents        []*PostContentFormated `json:"contents"`          // 帖子内容
	CommentCount    int64                  `json:"comment_count"`     // 评论数
	CollectionCount int64                  `json:"collection_count"`  // 收藏数
	ShareCount      int64                  `json:"share_count"`       // 分享数
	UpvoteCount     int64                  `json:"upvote_count"`      // 点赞数
	Visibility      PostVisibleT           `json:"visibility"`        // 可见性
	IsTop           int                    `json:"is_top"`            // 是否置顶
	IsEssence       int                    `json:"is_essence"`        // 是否精华
	IsLock          int                    `json:"is_lock"`           // 是否锁定
	LatestRepliedOn int64                  `json:"latest_replied_on"` // 最新回复时间
	CreatedOn       int64                  `json:"created_on"`        // 创建时间
	ModifiedOn      int64                  `json:"modified_on"`       // 修改时间
	Tags            map[string]int8        `json:"tags"`              // 标签
	IPLoc           string                 `json:"ip_loc"`            // IP地理位置
}

func (t PostVisibleT) ToOutValue() (res uint8) {
	switch t {
	case PostVisitPublic:
		res = 0
	case PostVisitPrivate:
		res = 1
	case PostVisitFriend:
		res = 2
	case PostVisitFollowing:
		res = 3
	default:
		res = 1
	}
	return
}

// Format 将帖子对象格式化为带有标签映射的格式化帖子。
func (p *Post) Format() *PostFormated {
	if p.Model != nil {
		tagsMap := map[string]int8{}
		for _, tag := range strings.Split(p.Tags, ",") {
			tagsMap[tag] = 1
		}
		return &PostFormated{
			ID:              p.ID,
			UserID:          p.UserID,
			User:            &UserFormated{},
			Contents:        []*PostContentFormated{},
			CommentCount:    p.CommentCount,
			CollectionCount: p.CollectionCount,
			ShareCount:      p.ShareCount,
			UpvoteCount:     p.UpvoteCount,
			Visibility:      p.Visibility,
			IsTop:           p.IsTop,
			IsEssence:       p.IsEssence,
			IsLock:          p.IsLock,
			LatestRepliedOn: p.LatestRepliedOn,
			CreatedOn:       p.CreatedOn,
			ModifiedOn:      p.ModifiedOn,
			Tags:            tagsMap,
			IPLoc:           p.IPLoc,
		}
	}

	return nil
}

// Create 创建帖子。
func (p *Post) Create(db *gorm.DB) (*Post, error) {
	err := db.Create(&p).Error
	return p, err
}

// Delete 根据ID删除帖子。
func (s *Post) Delete(db *gorm.DB) error {
	return db.Model(s).Where("id = ?", s.Model.ID).Updates(map[string]interface{}{
		"deleted_on": time.Now().Unix(),
		"is_del":     1,
	}).Error
}

// Get 根据条件获取单个帖子。
func (p *Post) Get(db *gorm.DB) (*Post, error) {
	var post Post
	if p.Model != nil && p.ID > 0 {
		db = db.Where("id = ? AND is_del = ?", p.ID, 0)
	} else {
		return nil, gorm.ErrRecordNotFound
	}

	err := db.First(&post).Error
	if err != nil {
		return &post, err
	}

	return &post, nil
}

// List 根据条件获取帖子列表。
func (p *Post) List(db *gorm.DB, conditions ConditionsT, offset, limit int) ([]*Post, error) {
	var posts []*Post
	var err error
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if p.UserID > 0 {
		db = db.Where("user_id = ?", p.UserID)
	}
	for k, v := range conditions {
		if k == "ORDER" {
			db = db.Order(v)
		} else {
			db = db.Where(k, v)
		}
	}

	if err = db.Where("is_del = ?", 0).Find(&posts).Error; err != nil {
		return nil, err
	}

	return posts, nil
}

// Fetch 根据条件获取帖子列表（扩展）。
func (p *Post) Fetch(db *gorm.DB, predicates Predicates, offset, limit int) ([]*Post, error) {
	var posts []*Post
	var err error
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if p.UserID > 0 {
		db = db.Where("user_id = ?", p.UserID)
	}
	for query, args := range predicates {
		if query == "ORDER" {
			db = db.Order(args[0])
		} else {
			db = db.Where(query, args...)
		}
	}

	if err = db.Where("is_del = ?", 0).Find(&posts).Error; err != nil {
		return nil, err
	}

	return posts, nil
}

// CountBy 根据条件计算帖子数量。
func (p *Post) CountBy(db *gorm.DB, predicates Predicates) (count int64, err error) {
	for query, args := range predicates {
		if query != "ORDER" {
			db = db.Where(query, args...)
		}
	}
	err = db.Model(p).Count(&count).Error
	return
}

// Count 根据条件统计帖子数量。
func (p *Post) Count(db *gorm.DB, conditions ConditionsT) (int64, error) {
	var count int64
	if p.UserID > 0 {
		db = db.Where("user_id = ?", p.UserID)
	}
	for k, v := range conditions {
		if k != "ORDER" {
			db = db.Where(k, v)
		}
	}
	if err := db.Model(p).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// Update 更新帖子信息。
func (p *Post) Update(db *gorm.DB) error {
	return db.Model(&Post{}).Where("id = ? AND is_del = ?", p.Model.ID, 0).Save(p).Error
}

// String 返回帖子可见性的字符串表示。
func (p PostVisibleT) String() string {
	switch p {
	case PostVisitPublic:
		return "public"
	case PostVisitPrivate:
		return "private"
	case PostVisitFriend:
		return "friend"
	default:
		return "unknown"
	}
}

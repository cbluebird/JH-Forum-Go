// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"context"
	"fmt"

	"unicode/utf8"

	"github.com/alimy/mir/v4"
	"github.com/gin-gonic/gin"
	api "JH-Forum/mirc/auto/api/v1"
	"JH-Forum/internal/conf"
	"JH-Forum/internal/core"
	"JH-Forum/internal/core/ms"
	"JH-Forum/internal/model/joint"
	"JH-Forum/internal/model/web"
	"JH-Forum/internal/servants/base"
	"JH-Forum/internal/servants/chain"
	"JH-Forum/pkg/xerror"
	"github.com/sirupsen/logrus"
)

var (
	// _MaxWhisperNumDaily 当日单用户私信总数限制（TODO 配置化、积分兑换等）
	_maxWhisperNumDaily int64 = 200
)

var (
	_ api.Core = (*coreSrv)(nil)
)

type coreSrv struct {
	api.UnimplementedCoreServant
	*base.DaoServant
	oss            core.ObjectStorageService
	wc             core.WebCache
	messagesExpire int64
	prefixMessages string
}

func (s *coreSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JWT()}
}

func (s *coreSrv) SyncSearchIndex(req *web.SyncSearchIndexReq) mir.Error {
	if req.User != nil && req.User.IsAdmin {
		fmt.Println("sync search index")
		s.PushAllPostToSearch()
	} else {
		fmt.Println(req.User.Username, req.User.IsAdmin)
		logrus.Warnf("sync search index need admin permision user: %#v", req.User)
	}
	return nil
}

func (s *coreSrv) GetUserInfo(req *web.UserInfoReq) (*web.UserInfoResp, mir.Error) {
	user, err := s.Ds.UserProfileByName(req.Username)
	if err != nil {
		logrus.Errorf("coreSrv.GetUserInfo occurs error[1]: %s", err)
		return nil, xerror.UnauthorizedAuthNotExist
	}
	follows, followings, err := s.Ds.GetFollowCount(user.ID)
	if err != nil {
		return nil, web.ErrGetFollowCountFailed
	}
	resp := &web.UserInfoResp{
		Id:          user.ID,
		Nickname:    user.Nickname,
		Username:    user.Username,
		Status:      user.Status,
		Avatar:      user.Avatar,
		Balance:     user.Balance,
		IsAdmin:     user.IsAdmin,
		CreatedOn:   user.CreatedOn,
		Follows:     follows,
		Followings:  followings,
		TweetsCount: user.TweetsCount,
	}
	if user.Phone != "" && len(user.Phone) == 11 {
		resp.Phone = user.Phone[0:3] + "****" + user.Phone[7:]
	}
	return resp, nil
}

func (s *coreSrv) GetMessages(req *web.GetMessagesReq) (res *web.GetMessagesResp, _ mir.Error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	// 尝试直接从缓存中获取数据
	key, ok := "", false
	if res, key, ok = s.messagesFromCache(req, limit, offset); ok {
		// logrus.Debugf("coreSrv.GetMessages from cache key:%s", key)
		return
	}
	messages, totalRows, err := s.Ds.GetMessages(req.Uid, req.Style, limit, offset)
	if err != nil {
		logrus.Errorf("Ds.GetMessages err[1]: %s", err)
		return nil, web.ErrGetMessagesFailed
	}
	for _, mf := range messages {
		// TODO: 优化处理这里的user获取逻辑以及错误处理
		if mf.SenderUserID > 0 {
			if user, err := s.Ds.GetUserByID(mf.SenderUserID); err == nil {
				mf.SenderUser = user.Format()
			}
		}
		if mf.Type == ms.MsgTypeWhisper && mf.ReceiverUserID != req.Uid {
			if user, err := s.Ds.GetUserByID(mf.ReceiverUserID); err == nil {
				mf.ReceiverUser = user.Format()
			}
		}
		// 好友申请消息不需要获取其他信息
		if mf.Type == ms.MsgTypeRequestingFriend {
			continue
		}
		if mf.PostID > 0 {
			post, err := s.GetTweetBy(mf.PostID)
			if err == nil {
				mf.Post = post
				if mf.CommentID > 0 {
					comment, err := s.Ds.GetCommentByID(mf.CommentID)
					if err == nil {
						mf.Comment = comment
						if mf.ReplyID > 0 {
							reply, err := s.Ds.GetCommentReplyByID(mf.ReplyID)
							if err == nil {
								mf.Reply = reply
							}
						}
					}
				}
			}
		}
	}
	if err != nil {
		logrus.Errorf("Ds.GetMessages err[2]: %s", err)
		return nil, web.ErrGetMessagesFailed
	}
	if err = s.PrepareMessages(req.Uid, messages); err != nil {
		logrus.Errorf("get messages err[3]: %s", err)
		return nil, web.ErrGetMessagesFailed
	}
	resp := joint.PageRespFrom(messages, req.Page, req.PageSize, totalRows)
	// 缓存处理
	base.OnCacheRespEvent(s.wc, key, resp, s.messagesExpire)
	return &web.GetMessagesResp{
		CachePageResp: joint.CachePageResp{
			Data: resp,
		},
	}, nil
}

func (s *coreSrv) ReadMessage(req *web.ReadMessageReq) mir.Error {
	message, err := s.Ds.GetMessageByID(req.ID)
	if err != nil {
		return web.ErrReadMessageFailed
	}
	if message.ReceiverUserID != req.Uid {
		return web.ErrNoPermission
	}
	if err = s.Ds.ReadMessage(message); err != nil {
		logrus.Errorf("Ds.ReadMessage err: %s", err)
		return web.ErrReadMessageFailed
	}
	// 缓存处理
	onMessageActionEvent(_messageActionRead, req.Uid)
	return nil
}

func (s *coreSrv) ReadAllMessage(req *web.ReadAllMessageReq) mir.Error {
	if err := s.Ds.ReadAllMessage(req.Uid); err != nil {
		logrus.Errorf("coreSrv.Ds.ReadAllMessage err: %s", err)
		return web.ErrReadMessageFailed
	}
	// 缓存处理
	onMessageActionEvent(_messageActionRead, req.Uid)
	return nil
}

func (s *coreSrv) SendUserWhisper(req *web.SendWhisperReq) mir.Error {
	// 不允许发送私信给自己
	if req.Uid == req.UserID {
		return web.ErrNoWhisperToSelf
	}
	// 今日频次限制
	ctx := context.Background()
	if count, _ := s.Redis.GetCountWhisper(ctx, req.Uid); count >= _maxWhisperNumDaily {
		return web.ErrTooManyWhisperNum
	}
	// 创建私信
	_, err := s.Ds.CreateMessage(&ms.Message{
		SenderUserID:   req.Uid,
		ReceiverUserID: req.UserID,
		Type:           ms.MsgTypeWhisper,
		Brief:          "给你发送新私信了",
		Content:        req.Content,
	})
	if err != nil {
		logrus.Errorf("Ds.CreateWhisper err: %s", err)
		return web.ErrSendWhisperFailed
	}
	// 缓存处理, 不需要处理错误
	onMessageActionEvent(_messageActionSendWhisper, req.Uid, req.UserID)
	// 写入当日（自然日）计数缓存
	s.Redis.IncrCountWhisper(ctx, req.Uid)

	return nil
}

func (s *coreSrv) GetCollections(req *web.GetCollectionsReq) (*web.GetCollectionsResp, mir.Error) {
	collections, err := s.Ds.GetUserPostCollections(req.UserId, (req.Page-1)*req.PageSize, req.PageSize)
	if err != nil {
		logrus.Errorf("Ds.GetUserPostCollections err: %s", err)
		return nil, web.ErrGetCollectionsFailed
	}
	totalRows, err := s.Ds.GetUserPostCollectionCount(req.UserId)
	if err != nil {
		logrus.Errorf("Ds.GetUserPostCollectionCount err: %s", err)
		return nil, web.ErrGetCollectionsFailed
	}
	var posts []*ms.Post
	for _, collection := range collections {
		posts = append(posts, collection.Post)
	}
	postsFormated, err := s.Ds.MergePosts(posts)
	if err != nil {
		logrus.Errorf("Ds.MergePosts err: %s", err)
		return nil, web.ErrGetCollectionsFailed
	}
	if err = s.PrepareTweets(req.UserId, postsFormated); err != nil {
		logrus.Errorf("get collections prepare tweets err: %s", err)
		return nil, web.ErrGetCollectionsFailed
	}
	resp := base.PageRespFrom(postsFormated, req.Page, req.PageSize, totalRows)
	return (*web.GetCollectionsResp)(resp), nil
}

func (s *coreSrv) GetStars(req *web.GetStarsReq) (*web.GetStarsResp, mir.Error) {
	stars, err := s.Ds.GetUserPostStars(req.UserId, req.PageSize, (req.Page-1)*req.PageSize)
	if err != nil {
		logrus.Errorf("Ds.GetUserPostStars err: %s", err)
		return nil, web.ErrGetStarsFailed
	}
	totalRows, err := s.Ds.GetUserPostStarCount(req.UserId)
	if err != nil {
		logrus.Errorf("Ds.GetUserPostStars err: %s", err)
		return nil, web.ErrGetStarsFailed
	}
	var posts []*ms.Post
	for _, star := range stars {
		posts = append(posts, star.Post)
	}
	postsFormated, err := s.Ds.MergePosts(posts)
	if err != nil {
		logrus.Errorf("Ds.MergePosts err: %s", err)
		return nil, web.ErrGetStarsFailed
	}
	resp := base.PageRespFrom(postsFormated, req.Page, req.PageSize, totalRows)
	return (*web.GetStarsResp)(resp), nil
}

func (s *coreSrv) ChangePassword(req *web.ChangePasswordReq) mir.Error {
	// 密码检查
	if err := checkPassword(req.Password); err != nil {
		return err
	}
	// 旧密码校验
	user := req.User
	if !validPassword(user.Password, req.OldPassword, req.User.Salt) {
		return web.ErrErrorOldPassword
	}
	// 更新入库
	user.Password, user.Salt = encryptPasswordAndSalt(req.Password)
	if err := s.Ds.UpdateUser(user); err != nil {
		logrus.Errorf("Ds.UpdateUser err: %s", err)
		return xerror.ServerError
	}
	return nil
}

func (s *coreSrv) SuggestTags(req *web.SuggestTagsReq) (*web.SuggestTagsResp, mir.Error) {
	tags, err := s.Ds.TagsByKeyword(req.Keyword)
	if err != nil {
		logrus.Errorf("Ds.GetTagsByKeyword err: %s", err)
		return nil, xerror.ServerError
	}
	resp := &web.SuggestTagsResp{}
	for _, t := range tags {
		resp.Suggests = append(resp.Suggests, t.Tag)
	}
	return resp, nil
}

func (s *coreSrv) SuggestUsers(req *web.SuggestUsersReq) (*web.SuggestUsersResp, mir.Error) {
	users, err := s.Ds.GetUsersByKeyword(req.Keyword)
	if err != nil {
		logrus.Errorf("Ds.GetUsersByKeyword err: %s", err)
		return nil, xerror.ServerError
	}
	resp := &web.SuggestUsersResp{}
	for _, user := range users {
		resp.Suggests = append(resp.Suggests, user.Username)
	}
	return resp, nil
}

func (s *coreSrv) ChangeNickname(req *web.ChangeNicknameReq) mir.Error {
	if utf8.RuneCountInString(req.Nickname) < 2 || utf8.RuneCountInString(req.Nickname) > 12 {
		return web.ErrNicknameLengthLimit
	}
	user := req.User
	user.Nickname = req.Nickname

	if err := s.Ds.UpdateUser(user); err != nil {
		logrus.Errorf("Ds.UpdateUser err: %s", err)
		return xerror.ServerError
	}
	// 缓存处理
	onChangeUsernameEvent(user.ID, user.Username)
	return nil
}

func (s *coreSrv) ChangeAvatar(req *web.ChangeAvatarReq) (xerr mir.Error) {
	defer func() {
		if xerr != nil {
			deleteOssObjects(s.oss, []string{req.Avatar})
		}
	}()

	if err := s.Ds.CheckAttachment(req.Avatar); err != nil {
		logrus.Errorf("Ds.CheckAttachment failed: %s", err)
		return xerror.InvalidParams
	}
	if err := s.oss.PersistObject(s.oss.ObjectKey(req.Avatar)); err != nil {
		logrus.Errorf("Ds.ChangeUserAvatar persist object failed: %s", err)
		return xerror.ServerError
	}
	user := req.User
	user.Avatar = req.Avatar
	if err := s.Ds.UpdateUser(user); err != nil {
		logrus.Errorf("Ds.UpdateUser failed: %s", err)
		return xerror.ServerError
	}
	// 缓存处理
	onChangeUsernameEvent(user.ID, user.Username)
	return nil
}

func (s *coreSrv) TweetCollectionStatus(req *web.TweetCollectionStatusReq) (*web.TweetCollectionStatusResp, mir.Error) {
	resp := &web.TweetCollectionStatusResp{
		Status: true,
	}
	if _, err := s.Ds.GetUserPostCollection(req.TweetId, req.Uid); err != nil {
		resp.Status = false
		return resp, nil
	}
	return resp, nil
}

func (s *coreSrv) TweetStarStatus(req *web.TweetStarStatusReq) (*web.TweetStarStatusResp, mir.Error) {
	resp := &web.TweetStarStatusResp{
		Status: true,
	}
	if _, err := s.Ds.GetUserPostStar(req.TweetId, req.Uid); err != nil {
		resp.Status = false
		return resp, nil
	}
	return resp, nil
}

func (s *coreSrv) messagesFromCache(req *web.GetMessagesReq, limit int, offset int) (res *web.GetMessagesResp, key string, ok bool) {
	key = fmt.Sprintf("%s%d:%s:%d:%d", s.prefixMessages, req.Uid, req.Style, limit, offset)
	if data, err := s.wc.Get(key); err == nil {
		ok, res = true, &web.GetMessagesResp{
			CachePageResp: joint.CachePageResp{
				JsonResp: data,
			},
		}
	}
	return
}

func newCoreSrv(s *base.DaoServant, oss core.ObjectStorageService, wc core.WebCache) api.Core {
	cs := conf.CacheSetting
	return &coreSrv{
		DaoServant:     s,
		oss:            oss,
		wc:             wc,
		messagesExpire: cs.MessagesExpire,
		prefixMessages: conf.PrefixMessages,
	}
}

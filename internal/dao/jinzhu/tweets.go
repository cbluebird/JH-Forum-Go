// 主要是关于微博（或类似社交平台）功能的实现
package jinzhu

import (
	"fmt"
	"strings"
	"time"

	"JH-Forum/internal/core"
	"JH-Forum/internal/core/cs"
	"JH-Forum/internal/core/ms"
	"JH-Forum/internal/dao/jinzhu/dbr"
	"JH-Forum/pkg/debug"
	"gorm.io/gorm"
)

var (
	_ core.TweetService       = (*tweetSrv)(nil)
	_ core.TweetManageService = (*tweetManageSrv)(nil)
	_ core.TweetHelpService   = (*tweetHelpSrv)(nil)

	_ core.TweetServantA       = (*tweetSrvA)(nil)
	_ core.TweetManageServantA = (*tweetManageSrvA)(nil)
	_ core.TweetHelpServantA   = (*tweetHelpSrvA)(nil)
)

// tweetSrv 实现了 core.TweetService 接口，用于处理微博的基本服务。
type tweetSrv struct {
	db *gorm.DB
}

// tweetManageSrv 实现了 core.TweetManageService 接口，管理微博的操作服务，包括创建、删除、锁定、置顶等。
type tweetManageSrv struct {
	cacheIndex core.CacheIndexService
	db         *gorm.DB
}

// tweetHelpSrv 实现了 core.TweetHelpService 接口，提供微博辅助功能，如数据整合、修复等。
type tweetHelpSrv struct {
	db *gorm.DB
}

// tweetSrvA 实现了 core.TweetServantA 接口，提供高级的微博服务功能。
type tweetSrvA struct {
	db *gorm.DB
}

// tweetManageSrvA 实现了 core.TweetManageServantA 接口，管理微博高级操作服务。
type tweetManageSrvA struct {
	db *gorm.DB
}

// tweetHelpSrvA 实现了 core.TweetHelpServantA 接口，提供高级的微博辅助功能服务。
type tweetHelpSrvA struct {
	db *gorm.DB
}

// newTweetService 创建并返回一个新的 tweetSrv 实例。
func newTweetService(db *gorm.DB) core.TweetService {
	return &tweetSrv{
		db: db,
	}
}

// newTweetManageService 创建并返回一个新的 tweetManageSrv 实例。

func newTweetManageService(db *gorm.DB, cacheIndex core.CacheIndexService) core.TweetManageService {
	return &tweetManageSrv{
		cacheIndex: cacheIndex,
		db:         db,
	}
}

// newTweetHelpService 创建并返回一个新的 tweetHelpSrv 实例。

func newTweetHelpService(db *gorm.DB) core.TweetHelpService {
	return &tweetHelpSrv{
		db: db,
	}
}

// newTweetServantA 创建并返回一个新的 tweetSrvA 实例。

func newTweetServantA(db *gorm.DB) core.TweetServantA {
	return &tweetSrvA{
		db: db,
	}
}

// newTweetManageServantA 创建并返回一个新的 tweetManageSrvA 实例。

func newTweetManageServantA(db *gorm.DB) core.TweetManageServantA {
	return &tweetManageSrvA{
		db: db,
	}
}

// newTweetHelpServantA 创建并返回一个新的 tweetHelpSrvA 实例。

func newTweetHelpServantA(db *gorm.DB) core.TweetHelpServantA {
	return &tweetHelpSrvA{
		db: db,
	}
}

// MergePosts 根据传入的 posts 切片，整合相关的用户和内容信息，返回整合后的 postsFormated 切片和错误。

func (s *tweetHelpSrv) MergePosts(posts []*ms.Post) ([]*ms.PostFormated, error) {
	postIds := make([]int64, 0, len(posts))
	userIds := make([]int64, 0, len(posts))
	for _, post := range posts {
		postIds = append(postIds, post.ID)
		userIds = append(userIds, post.UserID)
	}

	postContents, err := s.getPostContentsByIDs(postIds)
	if err != nil {
		return nil, err
	}

	users, err := s.getUsersByIDs(userIds)
	if err != nil {
		return nil, err
	}

	userMap := make(map[int64]*dbr.UserFormated, len(users))
	for _, user := range users {
		userMap[user.ID] = user.Format()
	}

	contentMap := make(map[int64][]*dbr.PostContentFormated, len(postContents))
	for _, content := range postContents {
		contentMap[content.PostID] = append(contentMap[content.PostID], content.Format())
	}

	// 数据整合
	postsFormated := make([]*dbr.PostFormated, 0, len(posts))
	for _, post := range posts {
		postFormated := post.Format()
		postFormated.User = userMap[post.UserID]
		postFormated.Contents = contentMap[post.ID]
		postsFormated = append(postsFormated, postFormated)
	}
	return postsFormated, nil
}

// RevampPosts 根据传入的 posts 切片，修复相关的用户和内容信息，返回修复后的 posts 切片和错误。

func (s *tweetHelpSrv) RevampPosts(posts []*ms.PostFormated) ([]*ms.PostFormated, error) {
	postIds := make([]int64, 0, len(posts))
	userIds := make([]int64, 0, len(posts))
	for _, post := range posts {
		postIds = append(postIds, post.ID)
		userIds = append(userIds, post.UserID)
	}

	postContents, err := s.getPostContentsByIDs(postIds)
	if err != nil {
		return nil, err
	}

	users, err := s.getUsersByIDs(userIds)
	if err != nil {
		return nil, err
	}

	userMap := make(map[int64]*dbr.UserFormated, len(users))
	for _, user := range users {
		userMap[user.ID] = user.Format()
	}

	contentMap := make(map[int64][]*dbr.PostContentFormated, len(postContents))
	for _, content := range postContents {
		contentMap[content.PostID] = append(contentMap[content.PostID], content.Format())
	}

	// 数据整合
	for _, post := range posts {
		post.User = userMap[post.UserID]
		post.Contents = contentMap[post.ID]
	}
	return posts, nil
}

// getPostContentsByIDs 根据传入的 IDs 获取对应的 post 内容信息切片。

func (s *tweetHelpSrv) getPostContentsByIDs(ids []int64) ([]*dbr.PostContent, error) {
	return (&dbr.PostContent{}).List(s.db, &dbr.ConditionsT{
		"post_id IN ?": ids,
		"ORDER":        "sort ASC",
	}, 0, 0)
}

// getUsersByIDs 根据传入的 IDs 获取对应的用户信息切片。
func (s *tweetHelpSrv) getUsersByIDs(ids []int64) ([]*dbr.User, error) {
	user := &dbr.User{}

	return user.List(s.db, &dbr.ConditionsT{
		"id IN ?": ids,
	}, 0, 0)
}

// CreatePostCollection 创建一个新的 post collection，并返回创建的 collection 和错误。

func (s *tweetManageSrv) CreatePostCollection(postID, userID int64) (*ms.PostCollection, error) {
	collection := &dbr.PostCollection{
		PostID: postID,
		UserID: userID,
	}

	return collection.Create(s.db)
}

// DeletePostCollection 删除指定的 post collection。
func (s *tweetManageSrv) DeletePostCollection(p *ms.PostCollection) error {
	return p.Delete(s.db)
}

func (s *tweetManageSrv) CreatePostContent(content *ms.PostContent) (*ms.PostContent, error) {
	return content.Create(s.db)
}

func (s *tweetManageSrv) CreateAttachment(obj *ms.Attachment) (int64, error) {
	attachment, err := obj.Create(s.db)
	return attachment.ID, err
}

func (s *tweetManageSrv) CreatePost(post *ms.Post) (*ms.Post, error) {
	post.LatestRepliedOn = time.Now().Unix()
	p, err := post.Create(s.db)
	if err != nil {
		return nil, err
	}
	s.cacheIndex.SendAction(core.IdxActCreatePost, post)
	return p, nil
}

func (s *tweetManageSrv) DeletePost(post *ms.Post) ([]string, error) {
	var mediaContents []string
	postId := post.ID
	postContent := &dbr.PostContent{}
	err := s.db.Transaction(
		func(tx *gorm.DB) error {
			if contents, err := postContent.MediaContentsByPostId(tx, postId); err == nil {
				mediaContents = contents
			} else {
				return err
			}

			// 删推文
			if err := post.Delete(tx); err != nil {
				return err
			}

			// 删内容
			if err := postContent.DeleteByPostId(tx, postId); err != nil {
				return err
			}

			// 删评论
			if contents, err := s.deleteCommentByPostId(tx, postId); err == nil {
				mediaContents = append(mediaContents, contents...)
			} else {
				return err
			}

			if tags := strings.Split(post.Tags, ","); len(tags) > 0 {
				// 删tag，宽松处理错误，有错误不会回滚
				deleteTags(tx, tags)
			}

			return nil
		},
	)

	if err != nil {
		return nil, err
	}

	s.cacheIndex.SendAction(core.IdxActDeletePost, post)
	return mediaContents, nil
}

func (s *tweetManageSrv) deleteCommentByPostId(db *gorm.DB, postId int64) ([]string, error) {
	comment := &dbr.Comment{}
	commentContent := &dbr.CommentContent{}

	// 获取推文的所有评论id
	commentIds, err := comment.CommentIdsByPostId(db, postId)
	if err != nil {
		return nil, err
	}

	// 获取评论的媒体内容
	mediaContents, err := commentContent.MediaContentsByCommentId(db, commentIds)
	if err != nil {
		return nil, err
	}

	// 删评论
	if err = comment.DeleteByPostId(db, postId); err != nil {
		return nil, err
	}

	// 删评论内容
	if err = commentContent.DeleteByCommentIds(db, commentIds); err != nil {
		return nil, err
	}

	// 删评论的评论
	if err = (&dbr.CommentReply{}).DeleteByCommentIds(db, commentIds); err != nil {
		return nil, err
	}

	return mediaContents, nil
}

// LockPost 锁定指定的 post。
func (s *tweetManageSrv) LockPost(post *ms.Post) error {
	post.IsLock = 1 - post.IsLock
	return post.Update(s.db)
}

func (s *tweetManageSrv) StickPost(post *ms.Post) error {
	post.IsTop = 1 - post.IsTop
	if err := post.Update(s.db); err != nil {
		return err
	}
	s.cacheIndex.SendAction(core.IdxActStickPost, post)
	return nil
}

func (s *tweetManageSrv) HighlightPost(userId int64, postId int64) (res int, err error) {
	var post dbr.Post
	tx := s.db.Begin()
	defer tx.Rollback()
	post.Get(tx)
	if err = tx.Where("id = ? AND is_del = 0", postId).First(&post).Error; err != nil {
		return
	}
	if post.UserID != userId {
		return 0, cs.ErrNoPermission
	}
	post.IsEssence = 1 - post.IsEssence
	if err = post.Update(tx); err != nil {
		return
	}
	tx.Commit()
	return post.IsEssence, nil
}

func (s *tweetManageSrv) VisiblePost(post *ms.Post, visibility cs.TweetVisibleType) (err error) {
	oldVisibility := post.Visibility
	post.Visibility = ms.PostVisibleT(visibility)
	// TODO: 这个判断是否可以不要呢
	if oldVisibility == ms.PostVisibleT(visibility) {
		return nil
	}
	// 私密推文 特殊处理
	if visibility == cs.TweetVisitPrivate {
		// 强制取消置顶
		// TODO: 置顶推文用户是否有权设置成私密？ 后续完善
		post.IsTop = 0
	}
	tx := s.db.Begin()
	defer tx.Rollback()
	if err = post.Update(tx); err != nil {
		return
	}
	// tag处理
	tags := strings.Split(post.Tags, ",")
	// TODO: 暂时宽松不处理错误，这里或许可以有优化，后续完善
	if oldVisibility == dbr.PostVisitPrivate {
		// 从私密转为非私密才需要重新创建tag
		createTags(tx, post.UserID, tags)
	} else if visibility == cs.TweetVisitPrivate {
		// 从非私密转为私密才需要删除tag
		deleteTags(tx, tags)
	}
	tx.Commit()
	s.cacheIndex.SendAction(core.IdxActVisiblePost, post)
	return
}

func (s *tweetManageSrv) UpdatePost(post *ms.Post) (err error) {
	if err = post.Update(s.db); err != nil {
		return
	}
	s.cacheIndex.SendAction(core.IdxActUpdatePost, post)
	return
}

func (s *tweetManageSrv) CreatePostStar(postID, userID int64) (*ms.PostStar, error) {
	star := &dbr.PostStar{
		PostID: postID,
		UserID: userID,
	}
	return star.Create(s.db)
}

func (s *tweetManageSrv) DeletePostStar(p *ms.PostStar) error {
	return p.Delete(s.db)
}

func (s *tweetSrv) GetPostByID(id int64) (*ms.Post, error) {
	post := &dbr.Post{
		Model: &dbr.Model{
			ID: id,
		},
	}
	return post.Get(s.db)
}

func (s *tweetSrv) GetPosts(conditions ms.ConditionsT, offset, limit int) ([]*ms.Post, error) {
	return (&dbr.Post{}).List(s.db, conditions, offset, limit)
}

func (s *tweetSrv) ListUserTweets(userId int64, style uint8, justEssence bool, limit, offset int) (res []*ms.Post, total int64, err error) {
	db := s.db.Model(&dbr.Post{}).Where("user_id = ?", userId)
	switch style {
	case cs.StyleUserTweetsAdmin:
		fallthrough
	case cs.StyleUserTweetsSelf:
		db = db.Where("visibility >= ?", cs.TweetVisitPrivate)
	case cs.StyleUserTweetsFriend:
		db = db.Where("visibility >= ?", cs.TweetVisitFriend)
	case cs.StyleUserTweetsFollowing:
		db = db.Where("visibility >= ?", cs.TweetVisitFollowing)
	case cs.StyleUserTweetsGuest:
		fallthrough
	default:
		db = db.Where("visibility >= ?", cs.TweetVisitPublic)
	}
	if justEssence {
		db = db.Where("is_essence=1")
	}
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if err = db.Order("is_top DESC, latest_replied_on DESC").Find(&res).Error; err != nil {
		return
	}
	return
}

func (s *tweetSrv) ListIndexNewestTweets(limit, offset int) (res []*ms.Post, total int64, err error) {
	db := s.db.Table(_post_).Where("visibility >= ?", cs.TweetVisitPublic)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if err = db.Order("is_top DESC, latest_replied_on DESC").Find(&res).Error; err != nil {
		return
	}
	return
}

func (s *tweetSrv) ListIndexHotsTweets(limit, offset int) (res []*ms.Post, total int64, err error) {
	db := s.db.Table(_post_).Joins(fmt.Sprintf("LEFT JOIN %s metric ON %s.id=metric.post_id", _post_metric_, _post_)).Where(fmt.Sprintf("visibility >= ? AND %s.is_del=0 AND metric.is_del=0", _post_), cs.TweetVisitPublic)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if err = db.Order("is_top DESC, metric.rank_score DESC, latest_replied_on DESC").Find(&res).Error; err != nil {
		return
	}
	return
}

func (s *tweetSrv) ListSyncSearchTweets(limit, offset int) (res []*ms.Post, total int64, err error) {
	db := s.db.Table(_post_).Where("visibility >= ?", cs.TweetVisitFriend)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if err = db.Find(&res).Error; err != nil {
		return
	}
	return
}

func (s *tweetSrv) ListFollowingTweets(userId int64, limit, offset int) (res []*ms.Post, total int64, err error) {
	beFriendIds, beFollowIds, xerr := s.getUserRelation(userId)
	if xerr != nil {
		return nil, 0, xerr
	}
	beFriendCount, beFollowCount := len(beFriendIds), len(beFollowIds)
	db := s.db.Model(&dbr.Post{})
	//可见性: 0私密 10充电可见 20订阅可见 30保留 40保留 50好友可见 60关注可见 70保留 80保留 90公开',
	switch {
	case beFriendCount > 0 && beFollowCount > 0:
		db = db.Where("user_id=? OR (visibility>=50 AND user_id IN(?)) OR (visibility>=60 AND user_id IN(?))", userId, beFriendIds, beFollowIds)
	case beFriendCount > 0 && beFollowCount == 0:
		db = db.Where("user_id=? OR (visibility>=50 AND user_id IN(?))", userId, beFriendIds)
	case beFriendCount == 0 && beFollowCount > 0:
		db = db.Where("user_id=? OR (visibility>=60 AND user_id IN(?))", userId, beFollowIds)
	case beFriendCount == 0 && beFollowCount == 0:
		db = db.Where("user_id = ?", userId)
	}
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if err = db.Order("is_top DESC, latest_replied_on DESC").Find(&res).Error; err != nil {
		return
	}
	return
}

func (s *tweetSrv) getUserRelation(userId int64) (beFriendIds []int64, beFollowIds []int64, err error) {
	if err = s.db.Table(_contact_).Where("friend_id=? AND status=2 AND is_del=0", userId).Select("user_id").Find(&beFriendIds).Error; err != nil {
		return
	}
	if err = s.db.Table(_following_).Where("user_id=? AND is_del=0", userId).Select("follow_id").Find(&beFollowIds).Error; err != nil {
		return
	}
	// 即是好友又是关注者，保留好友去除关注者
	for _, id := range beFriendIds {
		for i := 0; i < len(beFollowIds); i++ {
			// 找到item即删，数据库已经保证唯一性
			if beFollowIds[i] == id {
				lastIdx := len(beFollowIds) - 1
				beFollowIds[i] = beFollowIds[lastIdx]
				beFollowIds = beFollowIds[:lastIdx]
				break
			}
		}
	}
	return
}

func (s *tweetSrv) GetPostCount(conditions ms.ConditionsT) (int64, error) {
	return (&dbr.Post{}).Count(s.db, conditions)
}

func (s *tweetSrv) GetUserPostStar(postID, userID int64) (*ms.PostStar, error) {
	star := &dbr.PostStar{
		PostID: postID,
		UserID: userID,
	}
	return star.Get(s.db)
}

func (s *tweetSrv) GetUserPostStars(userID int64, limit int, offset int) ([]*ms.PostStar, error) {
	star := &dbr.PostStar{
		UserID: userID,
	}
	return star.List(s.db, &dbr.ConditionsT{
		"ORDER": s.db.NamingStrategy.TableName("PostStar") + ".id DESC",
	}, cs.RelationSelf, limit, offset)
}

func (s *tweetSrv) ListUserStarTweets(user *cs.VistUser, limit int, offset int) (res []*ms.PostStar, total int64, err error) {
	star := &dbr.PostStar{
		UserID: user.UserId,
	}
	if total, err = star.Count(s.db, user.RelTyp, &dbr.ConditionsT{}); err != nil {
		return
	}
	res, err = star.List(s.db, &dbr.ConditionsT{
		"ORDER": s.db.NamingStrategy.TableName("PostStar") + ".id DESC",
	}, user.RelTyp, limit, offset)
	return
}

func (s *tweetSrv) getUserTweets(db *gorm.DB, user *cs.VistUser, limit int, offset int) (res []*ms.Post, total int64, err error) {
	visibilities := []core.PostVisibleT{core.PostVisitPublic}
	switch user.RelTyp {
	case cs.RelationAdmin, cs.RelationSelf:
		visibilities = append(visibilities, core.PostVisitPrivate, core.PostVisitFriend)
	case cs.RelationFriend:
		visibilities = append(visibilities, core.PostVisitFriend)
	case cs.RelationGuest:
		fallthrough
	default:
		// nothing
	}
	db = db.Where("visibility IN ? AND is_del=0", visibilities)
	err = db.Count(&total).Error
	if err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	err = db.Order("latest_replied_on DESC").Find(&res).Error
	return
}

func (s *tweetSrv) ListUserMediaTweets(user *cs.VistUser, limit int, offset int) ([]*ms.Post, int64, error) {
	db := s.db.Table(_post_by_media_).Where("user_id=?", user.UserId)
	return s.getUserTweets(db, user, limit, offset)
}

func (s *tweetSrv) ListUserCommentTweets(user *cs.VistUser, limit int, offset int) ([]*ms.Post, int64, error) {
	db := s.db.Table(_post_by_comment_).Where("comment_user_id=?", user.UserId)
	return s.getUserTweets(db, user, limit, offset)
}

func (s *tweetSrv) GetUserPostStarCount(userID int64) (int64, error) {
	star := &dbr.PostStar{
		UserID: userID,
	}
	return star.Count(s.db, cs.RelationSelf, &dbr.ConditionsT{})
}

func (s *tweetSrv) GetUserPostCollection(postID, userID int64) (*ms.PostCollection, error) {
	star := &dbr.PostCollection{
		PostID: postID,
		UserID: userID,
	}
	return star.Get(s.db)
}

func (s *tweetSrv) GetUserPostCollections(userID int64, offset, limit int) ([]*ms.PostCollection, error) {
	collection := &dbr.PostCollection{
		UserID: userID,
	}

	return collection.List(s.db, &dbr.ConditionsT{
		"ORDER": s.db.NamingStrategy.TableName("PostCollection") + ".id DESC",
	}, offset, limit)
}

func (s *tweetSrv) GetUserPostCollectionCount(userID int64) (int64, error) {
	collection := &dbr.PostCollection{
		UserID: userID,
	}
	return collection.Count(s.db, &dbr.ConditionsT{})
}

func (s *tweetSrv) GetUserWalletBills(userID int64, offset, limit int) ([]*ms.WalletStatement, error) {
	statement := &dbr.WalletStatement{
		UserID: userID,
	}

	return statement.List(s.db, &dbr.ConditionsT{
		"ORDER": "id DESC",
	}, offset, limit)
}

func (s *tweetSrv) GetUserWalletBillCount(userID int64) (int64, error) {
	statement := &dbr.WalletStatement{
		UserID: userID,
	}
	return statement.Count(s.db, &dbr.ConditionsT{})
}

func (s *tweetSrv) GetPostAttatchmentBill(postID, userID int64) (*ms.PostAttachmentBill, error) {
	bill := &dbr.PostAttachmentBill{
		PostID: postID,
		UserID: userID,
	}

	return bill.Get(s.db)
}

func (s *tweetSrv) GetPostContentsByIDs(ids []int64) ([]*ms.PostContent, error) {
	return (&dbr.PostContent{}).List(s.db, &dbr.ConditionsT{
		"post_id IN ?": ids,
		"ORDER":        "sort ASC",
	}, 0, 0)
}

func (s *tweetSrv) GetPostContentByID(id int64) (*ms.PostContent, error) {
	return (&dbr.PostContent{
		Model: &dbr.Model{
			ID: id,
		},
	}).Get(s.db)
}

func (s *tweetSrvA) TweetInfoById(id int64) (*cs.TweetInfo, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) TweetItemById(id int64) (*cs.TweetItem, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) UserTweets(visitorId, userId int64) (cs.TweetList, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) ReactionByTweetId(userId int64, tweetId int64) (*cs.ReactionItem, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) UserReactions(userId int64, offset int, limit int) (cs.ReactionList, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) FavoriteByTweetId(userId int64, tweetId int64) (*cs.FavoriteItem, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) UserFavorites(userId int64, offset int, limit int) (cs.FavoriteList, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) AttachmentByTweetId(userId int64, tweetId int64) (*cs.AttachmentBill, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetManageSrvA) CreateAttachment(obj *cs.Attachment) (int64, error) {
	// TODO
	return 0, debug.ErrNotImplemented
}

func (s *tweetManageSrvA) CreateTweet(userId int64, req *cs.NewTweetReq) (*cs.TweetItem, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetManageSrvA) DeleteTweet(userId int64, tweetId int64) ([]string, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetManageSrvA) LockTweet(userId int64, tweetId int64) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetManageSrvA) StickTweet(userId int64, tweetId int64) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetManageSrvA) VisibleTweet(userId int64, visibility cs.TweetVisibleType) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetManageSrvA) CreateReaction(userId int64, tweetId int64) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetManageSrvA) DeleteReaction(userId int64, reactionId int64) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetManageSrvA) CreateFavorite(userId int64, tweetId int64) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetManageSrvA) DeleteFavorite(userId int64, favoriteId int64) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetHelpSrvA) RevampTweets(tweets cs.TweetList) (cs.TweetList, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetHelpSrvA) MergeTweets(tweets cs.TweetInfo) (cs.TweetList, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

package users

import (
	"context"

	"github.com/LazyEasyDev/LZApp/components"
)

type ReferralStatistic struct {
	RefUserID  uint64 `gorm:"column:ref_userid" json:"ref_userid"`
	RefChannel string `json:"ref_channel"`
	Count      int64  `json:"count"`
}

func ReferralStatistics(ctx context.Context, refUserID uint64, refChannel string, limit, offset int) ([]ReferralStatistic, int64, error) {
	query := components.GetDB().WithContext(ctx).Model(&User{}).
		Where("ref_userid IS NOT NULL AND ref_userid > 0")
	if refUserID != 0 {
		query = query.Where("ref_userid = ?", refUserID)
	}
	if refChannel != "" {
		query = query.Where("BINARY ref_channel = ?", refChannel)
	}
	grouped := query.Select("ref_userid, BINARY ref_channel AS ref_channel, COUNT(*) AS count").
		Group("ref_userid, BINARY ref_channel")
	var total int64
	if err := components.GetDB().WithContext(ctx).Table("(?) AS referral_groups", grouped).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]ReferralStatistic, 0)
	err := grouped.Order("ref_userid ASC, BINARY ref_channel ASC").Limit(limit).Offset(offset).Scan(&items).Error
	return items, total, err
}

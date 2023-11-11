package eventsub

import (
	"encoding/json"
	"fmt"
)

const (
	SubChannelFollow                                    = "channel.follow"
	SubChannelChannelPointsCustomRewardRedemptionAdd    = "channel.channel_points_custom_reward_redemption.add"
	SubChannelChannelPointsCustomRewardRedemptionUpdate = "channel.channel_points_custom_reward_redemption.update"
)

type Handler struct {
	OnChannelFollow                                    func(ChannelFollow)
	OnChannelChannelPointsCustomRewardRedemptionAdd    func(RewardAdd)
	OnChannelChannelPointsCustomRewardRedemptionUpdate func(RewardUpdate)
	OnAny                                              func(AnonymousNotification)
}

func (h *Handler) Delegate(subType string, data []byte) {
	switch subType {
	case SubChannelFollow:
		var typed ChannelFollowNotification
		if err := json.Unmarshal(data, &typed); err != nil {
			fmt.Printf("failed to handle %s\n", subType)
			return
		}
		if h.OnChannelFollow != nil {
			h.OnChannelFollow(typed.Event)
		}
	case SubChannelChannelPointsCustomRewardRedemptionAdd:
		var typed RewardAddNotification
		if err := json.Unmarshal(data, &typed); err != nil {
			fmt.Printf("failed to handle %s\n", subType)
			return
		}
		if h.OnChannelChannelPointsCustomRewardRedemptionAdd != nil {
			h.OnChannelChannelPointsCustomRewardRedemptionAdd(typed.Event)
		}
	case SubChannelChannelPointsCustomRewardRedemptionUpdate:
		var typed RewardUpdateNotification
		if err := json.Unmarshal(data, &typed); err != nil {
			fmt.Printf("failed to handle %s\n", subType)
			return
		}
		if h.OnChannelChannelPointsCustomRewardRedemptionUpdate != nil {
			h.OnChannelChannelPointsCustomRewardRedemptionUpdate(typed.Event)
		}
	default:
		fmt.Printf("unhandled: %s\n", subType)
	}
}

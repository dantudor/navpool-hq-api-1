package communityFund

import (
	"errors"
	"github.com/NavPool/navpool-hq-api/config"
	"github.com/NavPool/navpool-hq-api/database"
	"github.com/NavPool/navpool-hq-api/navpool"
	"github.com/NavPool/navpool-hq-api/service/account"
	"github.com/NavPool/navpool-hq-api/service/address"
	"github.com/NavPool/navpool-hq-api/service/communityFund/model"
	"github.com/getsentry/raven-go"
	"log"
)

var (
	ErrorUnableToGetProposalVotes       = errors.New("Unable to retrieve proposal votes")
	ErrorUnableToGetPaymentRequestVotes = errors.New("Unable to retrieve payment request votes")
	ErrorUnableToMatchVote              = errors.New("Unable to match vote")
)

func GetProposalVotes(user account.User) (votes []model.Vote, err error) {
	db, err := database.NewConnection()
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		return
	}
	defer database.Close(db)

	err = db.Where(&model.Vote{UserID: user.ID, Type: model.VoteTypeProposal}).Find(&votes).Error
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		err = ErrorUnableToGetProposalVotes
	}

	return
}

func UpdateProposalVotes(voteDtos []VoteDto, user account.User) (err error) {
	db, err := database.NewConnection()
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		return
	}
	defer database.Close(db)

	votes, err := GetProposalVotes(user)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		return err
	}

	tx := db.Begin()

	modifiedVotes := make([]model.Vote, 0)
	for _, voteDto := range voteDtos {
		vote, err := matchedVote(voteDto.Hash, model.VoteTypeProposal, votes)
		if err == nil {
			err = tx.Model(&vote).Updates(model.Vote{Choice: voteDto.Choice, Committed: false}).Error
		} else {
			raven.CaptureErrorAndWait(err, nil)
			newVote := &model.Vote{
				UserID:    user.ID,
				Type:      model.VoteTypeProposal,
				Hash:      voteDto.Hash,
				Choice:    voteDto.Choice,
				Committed: false,
			}
			err = tx.Create(newVote).Error
			vote = *newVote
		}

		if err != nil {
			raven.CaptureErrorAndWait(err, nil)
			tx.Rollback()
			return err
		}
		modifiedVotes = append(modifiedVotes, vote)
	}

	err = tx.Commit().Error
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		return
	}
	err = updatePoolVotes(modifiedVotes, user)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		return err
	}

	return nil
}

func GetPaymentRequestVotes(user account.User) (votes []model.Vote, err error) {
	db, err := database.NewConnection()
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		return
	}
	defer database.Close(db)

	err = db.Where(&model.Vote{UserID: user.ID, Type: model.VoteTypePaymentRequest}).Find(&votes).Error
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		err = ErrorUnableToGetPaymentRequestVotes
	}

	return
}

func UpdatePaymentRequestVotes(voteDtos []VoteDto, user account.User) (err error) {
	db, err := database.NewConnection()
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		return
	}
	defer database.Close(db)

	votes, err := GetPaymentRequestVotes(user)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		return err
	}

	tx := db.Begin()

	modifiedVotes := make([]model.Vote, 0)
	for _, voteDto := range voteDtos {
		vote, err := matchedVote(voteDto.Hash, model.VoteTypePaymentRequest, votes)
		if err == nil {
			err = tx.Model(&vote).Updates(model.Vote{Choice: voteDto.Choice, Committed: false}).Error
		} else {
			raven.CaptureErrorAndWait(err, nil)
			newVote := &model.Vote{
				UserID:    user.ID,
				Type:      model.VoteTypePaymentRequest,
				Hash:      voteDto.Hash,
				Choice:    voteDto.Choice,
				Committed: false,
			}
			err = tx.Create(newVote).Error
			vote = *newVote
		}

		if err != nil {
			raven.CaptureErrorAndWait(err, nil)
			tx.Rollback()
			return err
		}
		modifiedVotes = append(modifiedVotes, vote)
	}
	err = tx.Commit().Error
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		return
	}

	log.Printf("%d votes have been updated", len(modifiedVotes))
	err = updatePoolVotes(modifiedVotes, user)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		return err
	}

	return nil
}

func matchedVote(hash string, voteType model.VoteType, votes []model.Vote) (matched model.Vote, err error) {
	for _, vote := range votes {
		if hash == vote.Hash && voteType == vote.Type {
			return vote, nil
		}
	}

	err = ErrorUnableToMatchVote

	return
}

func updatePoolVotes(votes []model.Vote, user account.User) (err error) {
	db, err := database.NewConnection()
	if err != nil {
		return
	}
	defer database.Close(db)

	addresses, err := address.GetAddresses(user)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		return err
	}

	voteOptions := map[model.VoteChoice]string{
		"YES":     "yes",
		"NO":      "no",
		"ABSTAIN": "remove",
	}

	poolApi, err := navpool.NewPoolApi(config.Get().Pool.Url, config.Get().SelectedNetwork)
	for _, address := range addresses {
		for _, vote := range votes {
			if vote.Type == "PROPOSAL" {
				err = poolApi.ProposalVote(address.SpendingAddress, vote.Hash, voteOptions[vote.Choice])
			} else {
				err = poolApi.PaymentRequestVote(address.SpendingAddress, vote.Hash, voteOptions[vote.Choice])
			}
			if err == nil {
				vote.Committed = true
				db.Save(&vote)
			}
		}
	}

	return
}

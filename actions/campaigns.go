package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	valid "github.com/asaskevich/govalidator"
	"github.com/gin-gonic/gin"
	"github.com/news-maily/api/entities"
	"github.com/news-maily/api/queue"
	"github.com/news-maily/api/routes/middleware"
	"github.com/news-maily/api/storage"
	"github.com/news-maily/api/utils/pagination"
)

type sendCampaignParams struct {
	Ids    []int64 `form:"list_id[]" valid:"required"`
	Source string  `form:"source" valid:"email,required~Email is blank or in invalid format"`
}

func StartCampaign(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"reason": "Id must be an integer",
		})
		return
	}

	params := &sendCampaignParams{}
	c.Bind(params)

	v, err := valid.ValidateStruct(params)
	if err != nil || !v {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"reason": err.Error(),
		})
		return
	}

	templateData := c.PostFormMap("default_template_data")

	u := middleware.GetUser(c)

	campaign, err := storage.GetCampaign(c, id, u.Id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"reason": "Campaign not found",
		})
		return
	}

	if campaign.Status != entities.StatusDraft {
		c.JSON(http.StatusBadRequest, gin.H{
			"reason": fmt.Sprintf(`Campaign has a status of '%s', cannot start the campaign.`, campaign.Status),
		})
		return
	}

	sesKeys, err := storage.GetSesKeys(c, u.Id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"reason": "Amazon Ses keys are not set.",
		})
		return
	}

	lists, err := storage.GetListsByIDs(c, u.Id, params.Ids)
	if err != nil || len(lists) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"reason": "Subscriber lists are not found.",
		})
		return
	}

	msg, err := json.Marshal(entities.SendCampaignParams{
		ListIDs:      params.Ids,
		Source:       params.Source,
		CampaignID:   campaign.Id,
		UserID:       campaign.UserId,
		TemplateName: campaign.TemplateName,
		TemplateData: templateData,
		SesKeys:      *sesKeys,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"reason": "Unable to publish campaign.",
		})
		return
	}

	err = queue.Publish(c, entities.CampaignsTopic, msg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"reason": "Unable to publish campaign.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reason": "The campaign has started. You can track the progress in the campaign details page.",
	})
	return
}

func GetCampaigns(c *gin.Context) {
	val, ok := c.Get("pagination")
	if !ok {
		c.AbortWithError(http.StatusInternalServerError, errors.New("cannot create pagination object"))
		return
	}

	p, ok := val.(*pagination.Pagination)
	if !ok {
		c.AbortWithError(http.StatusInternalServerError, errors.New("cannot cast pagination object"))
		return
	}

	storage.GetCampaigns(c, middleware.GetUser(c).Id, p)
	c.JSON(http.StatusOK, p)
}

func GetCampaign(c *gin.Context) {
	if id, err := strconv.ParseInt(c.Param("id"), 10, 64); err == nil {
		if campaign, err := storage.GetCampaign(c, id, middleware.GetUser(c).Id); err == nil {
			c.JSON(http.StatusOK, campaign)
			return
		}

		c.JSON(http.StatusNotFound, gin.H{
			"reason": "Campaign not found",
		})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{
		"reason": "Id must be an integer",
	})
	return
}

func PostCampaign(c *gin.Context) {
	name, templateName := c.PostForm("name"), c.PostForm("template_name")
	user := middleware.GetUser(c)

	_, err := storage.GetCampaignByName(c, name, middleware.GetUser(c).Id)
	if err == nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"reason": "Campaign with that name already exists",
		})
		return
	}

	campaign := &entities.Campaign{
		Name:         name,
		UserId:       user.Id,
		TemplateName: templateName,
		Status:       entities.StatusDraft,
	}

	if !campaign.Validate() {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"reason": "Invalid data",
			"errors": campaign.Errors,
		})
		return
	}

	err = storage.CreateCampaign(c, campaign)

	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"reason": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, campaign)
	return
}

func PutCampaign(c *gin.Context) {
	if id, err := strconv.ParseInt(c.Param("id"), 10, 64); err == nil {
		user := middleware.GetUser(c)

		campaign, err := storage.GetCampaign(c, id, user.Id)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"reason": "Campaign not found",
			})
			return
		}

		name, templateName := c.PostForm("name"), c.PostForm("template_name")

		campaign2, err := storage.GetCampaignByName(c, name, middleware.GetUser(c).Id)
		if err == nil && campaign.Id != campaign2.Id {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"reason": "Campaign with that name already exists",
			})
			return
		}

		campaign.Name = name
		campaign.TemplateName = templateName

		if !campaign.Validate() {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"reason": "Invalid data",
				"errors": campaign.Errors,
			})
			return
		}

		err = storage.UpdateCampaign(c, campaign)

		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"reason": err.Error(),
			})
			return
		}

		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{
		"reason": "Id must be an integer",
	})
	return
}

func DeleteCampaign(c *gin.Context) {
	if id, err := strconv.ParseInt(c.Param("id"), 10, 64); err == nil {
		user := middleware.GetUser(c)

		_, err := storage.GetCampaign(c, id, user.Id)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"reason": "Campaign not found",
			})
			return
		}

		err = storage.DeleteCampaign(c, id, user.Id)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"reason": err.Error(),
			})
			return
		}

		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{
		"reason": "Id must be an integer",
	})
	return
}

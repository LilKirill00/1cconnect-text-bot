package client

import (
	"connect-text-bot/internal/database"
	"connect-text-bot/internal/us"
	"context"

	"github.com/google/uuid"
	"github.com/hooklift/gowsdl/soap"
)

// Создание заявки на обслуживание
func ServiceRequestAdd(ctx context.Context, soapcl *soap.Client, userID, lineID uuid.UUID, ticket database.Ticket) (content map[string]string, err error) {
	service := us.NewPartnerWebAPI2PortType(soapcl)

	serviceRequestAdd, err := service.ServiceRequestAddContext(
		ctx,
		&us.ServiceRequestAdd{
			Xs: "http://www.w3.org/2001/XMLSchema",
			Params: &us.Params{
				Property: []us.ParamsProperty{
					formProperty("ServiceRequestChannelID", us.XsString, ticket.ChannelID.String()),
					formProperty("ServiceLineKindID", us.XsString, lineID.String()),
					formProperty("ServiceKindID", us.XsString, ticket.Service.ID.String()),
					formProperty("ServiceRequestTypeID", us.XsString, ticket.ServiceType.ID.String()),
					formProperty("UserID", us.XsString, userID.String()),
					formProperty("ExecutorID", us.XsString, ticket.Executor.ID.String()),
					formProperty("Description", us.XsString, ticket.Description),
					formProperty("Summary", us.XsString, ticket.Theme),
				},
			},
		},
	)
	if err != nil {
		return
	}

	r, err := serviceRequestAdd.Return_.GetResult()
	if err != nil {
		return
	}

	content = make(map[string]string)
	for _, v := range r {
		content[v.Name] = v.Value.Text
	}

	return
}

// сформировать поле для запроса
func formProperty(name, valueType, valueText string) us.ParamsProperty {
	return us.ParamsProperty{
		Name: name,
		Value: us.PropertyValue{
			Type: valueType,
			Text: valueText,
		},
	}
}

package turso

import (
	"errors"
	"fmt"
	"net/http"
)

type Instance struct {
	Uuid     string
	Name     string
	Type     string
	Region   string
	Hostname string
}

type InstancesClient client

func (i *InstancesClient) List(db string) ([]Instance, error) {
	r, err := i.client.Get(fmt.Sprintf("v2/databases/%s/instances", db), nil)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response with status code %d", r.StatusCode)
	}

	type ListResponse struct{ Instances []Instance }
	resp, err := unmarshal[ListResponse](r)
	if err != nil {
		return nil, err
	}

	return resp.Instances, nil
}

func (i *InstancesClient) Delete(db, instance string) error {
	r, err := i.client.Delete(fmt.Sprintf("v2/databases/%s/instances/%s", db, instance), nil)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if r.StatusCode == http.StatusBadRequest {
		body, _ := unmarshal[struct{ Error string }](r)
		return errors.New(body.Error)
	}

	if r.StatusCode == http.StatusNotFound {
		return errors.New("instance not found")
	}

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("response with status code %d", r.StatusCode)
	}

	return nil
}

func (d *InstancesClient) Create(dbName, password, region, image string) (*Instance, error) {
	type Body struct{ Password, Region, Image string }
	body, err := marshal(Body{password, region, image})
	if err != nil {
		return nil, fmt.Errorf("could not serialize request body: %w", err)
	}

	url := fmt.Sprintf("/v2/databases/%s/instances", dbName)
	res, err := d.client.Post(url, body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, parseResponseError(res)
	}

	instance, err := unmarshal[*Instance](res)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize response: %w", err)
	}

	return instance, nil
}
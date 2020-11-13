package auth0

import (
	"log"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"

	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

func newLogStream() *schema.Resource {
	return &schema.Resource{

		Create: createLogStream,
		Read:   readLogStream,
		Update: updateLogStream,
		Delete: deleteLogStream,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"type": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice([]string{
					"eventbridge", "eventgrid", "http", "datadog", "splunk"}, true),
				ForceNew:    true,
				Description: "Type of the LogStream, which indicates the Sink provider",
			},
			"status": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateFunc: validation.StringInSlice([]string{
					"active", "paused", "suspended"}, false),
				Description: "Status of the LogStream",
			},
			// - `eventbridge` requires `awsAccountId`, and `awsRegion`
			"aws_account_id": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ForceNew:      true,
				ConflictsWith: []string{"azure_subscription_id", "http_endpoint", "datadog_api_key", "splunk_token"},
				RequiredWith:  []string{"aws_region"},
			},
			"aws_region": {
				Type:         schema.TypeString,
				Optional:     true,
				Sensitive:    true,
				ForceNew:     true,
				RequiredWith: []string{"aws_account_id"},
			},
			"aws_partner_event_source": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the Partner Event Source to be used with AWS, if the type is 'eventbridge'",
			},
			// - `eventgrid` requires `azureSubscriptionId`, `azureResourceGroup`, and `azureRegion`
			"azure_subscription_id": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ForceNew:      true,
				ConflictsWith: []string{"aws_account_id", "http_endpoint", "datadog_api_key", "splunk_token"},
				RequiredWith:  []string{"azure_resource_group", "azure_region"},
			},
			"azure_resource_group": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ForceNew:      true,
				ConflictsWith: []string{"aws_account_id"},
				RequiredWith:  []string{"azure_subscription_id", "azure_region"},
			},
			"azure_region": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ForceNew:      true,
				ConflictsWith: []string{"aws_account_id"},
				RequiredWith:  []string{"azure_subscription_id", "azure_resource_group"},
			},
			"azure_partner_topic": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the Partner Topic to be used with Azure, if the type is 'eventgrid'",
			},
			// - `http` requires `httpEndpoint`, `httpContentType`, `httpContentFormat`, and `httpAuthorization`
			"http_content_format": {
				Type:         schema.TypeString,
				Optional:     true,
				RequiredWith: []string{"http_endpoint", "http_authorization", "http_content_type"},
				ValidateFunc: validation.StringInSlice([]string{
					"JSONLINES", "JSONARRAY"}, false),
			},
			"http_content_type": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "HTTP Content Type",
				RequiredWith: []string{"http_endpoint", "http_authorization", "http_content_format"},
			},
			"http_endpoint": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   "HTTP endpoint",
				RequiredWith:  []string{"http_content_format", "http_authorization", "http_content_type"},
				ConflictsWith: []string{"aws_account_id", "azure_subscription_id", "datadog_api_key", "splunk_token"},
			},
			"http_authorization": {
				Type:         schema.TypeString,
				Optional:     true,
				Sensitive:    true,
				RequiredWith: []string{"http_endpoint", "http_content_format", "http_content_type"},
			},
			"http_custom_headers": {
				Type:          schema.TypeSet,
				Elem:          &schema.Schema{Type: schema.TypeString},
				Optional:      true,
				Description:   "custom HTTP headers",
				ConflictsWith: []string{"aws_account_id", "azure_subscription_id", "datadog_api_key", "splunk_token"},
			},
			// - `datadog` requires `datadogRegion`, and `datadogApiKey`
			"datadog_region": {
				Type:          schema.TypeString,
				Optional:      true,
				RequiredWith:  []string{"datadog_api_key"},
				ConflictsWith: []string{"aws_account_id", "azure_subscription_id", "http_endpoint", "splunk_token"},
			},
			"datadog_api_key": {
				Type:         schema.TypeString,
				Optional:     true,
				Sensitive:    true,
				ForceNew:     true,
				RequiredWith: []string{"datadog_region"},
			},
			// - `splunk` requires `splunkDomain`, `splunkToken`, `splunkPort`, and `splunkSecure`
			"splunk_domain": {
				Type:         schema.TypeString,
				Optional:     true,
				RequiredWith: []string{"splunk_token", "splunk_port", "splunk_secure"},
			},
			"splunk_token": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				RequiredWith:  []string{"splunk_domain", "splunk_port", "splunk_secure"},
				ConflictsWith: []string{"aws_account_id", "azure_subscription_id", "http_endpoint", "datadog_api_key"},
			},
			"splunk_port": {
				Type:         schema.TypeString,
				Optional:     true,
				RequiredWith: []string{"splunk_domain", "splunk_token", "splunk_secure"},
			},
			"splunk_secure": {
				Type:         schema.TypeBool,
				Optional:     true,
				RequiredWith: []string{"splunk_domain", "splunk_port", "splunk_token"},
			},
		},
	}
}

func createLogStream(d *schema.ResourceData, m interface{}) error {
	api := m.(*management.Management)
	ls := expandLogStream(d)
	if err := api.LogStream.Create(ls); err != nil {
		return err
	}
	d.SetId(auth0.StringValue(ls.ID))
	return readLogStream(d, m)
}

func readLogStream(d *schema.ResourceData, m interface{}) error {
	api := m.(*management.Management)
	ls, err := api.LogStream.Read(d.Id())
	if err != nil {
		if mErr, ok := err.(management.Error); ok {
			if mErr.Status() == http.StatusNotFound {
				d.SetId("")
				return nil
			}
		}
		return err
	}

	d.SetId(auth0.StringValue(ls.ID))
	d.Set("name", ls.Name)
	d.Set("status", ls.Status)
	d.Set("type", ls.Type)
	flattenLogStreamSink(d, ls.Sink)
	return nil
}

func updateLogStream(d *schema.ResourceData, m interface{}) error {
	c := expandLogStream(d)
	api := m.(*management.Management)
	err := api.LogStream.Update(d.Id(), c)
	if err != nil {
		return err
	}
	return readLogStream(d, m)
}

func deleteLogStream(d *schema.ResourceData, m interface{}) error {
	api := m.(*management.Management)
	err := api.LogStream.Delete(d.Id())
	if err != nil {
		if mErr, ok := err.(management.Error); ok {
			if mErr.Status() == http.StatusNotFound {
				d.SetId("")
				return nil
			}
		}
	}
	return err
}

func flattenLogStreamSink(d ResourceData, sink interface{}) []interface{} {

	var m interface{}

	switch o := sink.(type) {
	case *management.LogStreamSinkAmazonEventBridge:
		flattenLogStreamEventBridgeSink(d, o)
	case *management.LogStreamSinkAzureEventGrid:
		flattenLogStreamEventGridSink(d, o)
	case *management.LogStreamSinkHTTP:
		flattenLogStreamHTTPSink(d, o)
	case *management.LogStreamSinkDatadog:
		flattenLogStreamDatadogSink(d, o)
	case *management.LogStreamSinkSplunk:
		flattenLogStreamSplunkSink(d, o)
	}
	return []interface{}{m}
}

func flattenLogStreamEventBridgeSink(d ResourceData, o *management.LogStreamSinkAmazonEventBridge) {
	d.Set("aws_account_id", o.GetAccountID())
	d.Set("aws_region", o.GetRegion())
	d.Set("aws_partner_event_source", o.GetPartnerEventSource())
}

func flattenLogStreamEventGridSink(d ResourceData, o *management.LogStreamSinkAzureEventGrid) {
	d.Set("azure_subscription_id", o.GetSubscriptionID())
	d.Set("azure_resource_group", o.GetResourceGroup())
	d.Set("azure_region", o.GetRegion())
	d.Set("azure_partner_topic", o.GetPartnerTopic())
}

func flattenLogStreamHTTPSink(d ResourceData, o *management.LogStreamSinkHTTP) {
	d.Set("http_endpoint", o.GetEndpoint())
	d.Set("http_contentFormat", o.GetContentFormat())
	d.Set("http_contentType", o.GetContentType())
	d.Set("http_authorization", o.GetAuthorization())
	d.Set("http_custom_headers", o.CustomHeaders)
}

func flattenLogStreamDatadogSink(d ResourceData, o *management.LogStreamSinkDatadog) {
	d.Set("datadog_region", o.GetRegion())
	d.Set("datadog_api_key", o.GetAPIKey())
}

func flattenLogStreamSplunkSink(d ResourceData, o *management.LogStreamSinkSplunk) {
	d.Set("splunk_domain", o.GetDomain())
	d.Set("splunk_token", o.GetToken())
	d.Set("splunk_port", o.GetPort())
	d.Set("splunk_secure", o.GetSecure())
}
func expandLogStream(d ResourceData) *management.LogStream {

	ls := &management.LogStream{
		Name:   String(d, "name", IsNewResource()),
		Type:   String(d, "type", IsNewResource()),
		Status: String(d, "status"),
	}

	s := d.Get("type").(string)
	switch s {
	case management.LogStreamTypeAmazonEventBridge:
		ls.Sink = expandLogStreamEventBridgeSink(d)
	case management.LogStreamTypeAzureEventGrid:
		ls.Sink = expandLogStreamEventGridSink(d)
	case management.LogStreamTypeHTTP:
		ls.Sink = expandLogStreamHTTPSink(d)
	case management.LogStreamTypeDatadog:
		ls.Sink = expandLogStreamDatadogSink(d)
	case management.LogStreamTypeSplunk:
		ls.Sink = expandLogStreamSplunkSink(d)
	default:
		log.Printf("[WARN]: Unsupported log stream sink %s", s)
		log.Printf("[WARN]: Raise an issue with the auth0 provider in order to support it:")
		log.Printf("[WARN]: 	https://github.com/alexkappa/terraform-provider-auth0/issues/new")
	}

	return ls
}

func expandLogStreamEventBridgeSink(d ResourceData) *management.LogStreamSinkAmazonEventBridge {
	o := &management.LogStreamSinkAmazonEventBridge{
		AccountID:          String(d, "aws_account_id"),
		Region:             String(d, "aws_region"),
		PartnerEventSource: String(d, "aws_partner_event_source"),
	}
	return o
}

func expandLogStreamEventGridSink(d ResourceData) *management.LogStreamSinkAzureEventGrid {
	o := &management.LogStreamSinkAzureEventGrid{
		SubscriptionID: String(d, "azure_subscription_id"),
		ResourceGroup:  String(d, "azure_resource_group"),
		Region:         String(d, "azure_region"),
		PartnerTopic:   String(d, "azure_partner_topic"),
	}
	return o
}

func expandLogStreamHTTPSink(d ResourceData) *management.LogStreamSinkHTTP {
	o := &management.LogStreamSinkHTTP{
		ContentFormat: String(d, "http_content_format"),
		ContentType:   String(d, "http_content_type"),
		Endpoint:      String(d, "http_endpoint"),
		Authorization: String(d, "http_authorization"),
		CustomHeaders: Set(d, "http_custom_headers").List(),
	}
	return o
}
func expandLogStreamDatadogSink(d ResourceData) *management.LogStreamSinkDatadog {
	o := &management.LogStreamSinkDatadog{
		Region: String(d, "datadog_region"),
		APIKey: String(d, "datadog_api_key"),
	}
	return o
}
func expandLogStreamSplunkSink(d ResourceData) *management.LogStreamSinkSplunk {
	o := &management.LogStreamSinkSplunk{
		Domain: String(d, "splunk_domain"),
		Token:  String(d, "splunk_token"),
		Port:   String(d, "splunk_port"),
		Secure: Bool(d, "splunk_secure"),
	}
	return o
}

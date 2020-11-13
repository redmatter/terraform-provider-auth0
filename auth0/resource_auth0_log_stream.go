package auth0

import (
	"log"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"

	"gopkg.in/auth0.v4"
	"gopkg.in/auth0.v4/management"
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
	case *management.EventBridgeSink:
		flattenLogStreamEventBridgeSink(d, o)
	case *management.EventGridSink:
		flattenLogStreamEventGridSink(d, o)
	case *management.HTTPSink:
		flattenLogStreamHTTPSink(d, o)
	case *management.DatadogSink:
		flattenLogStreamDatadogSink(d, o)
	case *management.SplunkSink:
		flattenLogStreamSplunkSink(d, o)
	}
	return []interface{}{m}
}

func flattenLogStreamEventBridgeSink(d ResourceData, o *management.EventBridgeSink) {
	d.Set("aws_account_id", o.GetAWSAccountID())
	d.Set("aws_region", o.GetAWSRegion())
	d.Set("aws_partner_event_source", o.GetAWSPartnerEventSource())
}

func flattenLogStreamEventGridSink(d ResourceData, o *management.EventGridSink) {
	d.Set("azure_subscription_id", o.GetAzureSubscriptionID())
	d.Set("azure_resource_group", o.GetAzureResourceGroup())
	d.Set("azure_region", o.GetAzureRegion())
	d.Set("azure_partner_topic", o.GetAzurePartnerTopic())
}

func flattenLogStreamHTTPSink(d ResourceData, o *management.HTTPSink) {
	d.Set("http_endpoint", o.GetHTTPEndpoint())
	d.Set("http_contentFormat", o.GetHTTPContentFormat())
	d.Set("http_contentType", o.GetHTTPContentType())
	d.Set("http_authorization", o.GetHTTPAuthorization())
	d.Set("http_custom_headers", o.HTTPCustomHeaders)
}

func flattenLogStreamDatadogSink(d ResourceData, o *management.DatadogSink) {
	d.Set("datadog_region", o.GetDatadogRegion())
	d.Set("datadog_api_key", o.GetDatadogAPIKey())
}

func flattenLogStreamSplunkSink(d ResourceData, o *management.SplunkSink) {
	d.Set("splunk_domain", o.GetSplunkDomain())
	d.Set("splunk_token", o.GetSplunkToken())
	d.Set("splunk_port", o.GetSplunkPort())
	d.Set("splunk_secure", o.GetSplunkSecure())
}
func expandLogStream(d ResourceData) *management.LogStream {

	ls := &management.LogStream{
		Name:   String(d, "name", IsNewResource()),
		Type:   String(d, "type", IsNewResource()),
		Status: String(d, "status"),
	}

	s := d.Get("type").(string)
	switch s {
	case management.LogStreamSinkEventBridge:
		ls.Sink = expandLogStreamEventBridgeSink(d)
	case management.LogStreamSinkEventGrid:
		ls.Sink = expandLogStreamEventGridSink(d)
	case management.LogStreamSinkHTTP:
		ls.Sink = expandLogStreamHTTPSink(d)
	case management.LogStreamSinkDatadog:
		ls.Sink = expandLogStreamDatadogSink(d)
	case management.LogStreamSinkSplunk:
		ls.Sink = expandLogStreamSplunkSink(d)
	default:
		log.Printf("[WARN]: Raise an issue with the auth0 provider in order to support it:")
		log.Printf("[WARN]: 	https://github.com/alexkappa/terraform-provider-auth0/issues/new")
	}

	return ls
}

func expandLogStreamEventBridgeSink(d ResourceData) *management.EventBridgeSink {
	o := &management.EventBridgeSink{
		AWSAccountID:          String(d, "aws_account_id"),
		AWSRegion:             String(d, "aws_region"),
		AWSPartnerEventSource: String(d, "aws_partner_event_source"),
	}
	return o
}

func expandLogStreamEventGridSink(d ResourceData) *management.EventGridSink {
	o := &management.EventGridSink{
		AzureSubscriptionID: String(d, "azure_subscription_id"),
		AzureResourceGroup:  String(d, "azure_resource_group"),
		AzureRegion:         String(d, "azure_region"),
		AzurePartnerTopic:   String(d, "azure_partner_topic"),
	}
	return o
}

func expandLogStreamHTTPSink(d ResourceData) *management.HTTPSink {
	o := &management.HTTPSink{
		HTTPContentFormat: String(d, "http_content_format"),
		HTTPContentType:   String(d, "http_content_type"),
		HTTPEndpoint:      String(d, "http_endpoint"),
		HTTPAuthorization: String(d, "http_authorization"),
		HTTPCustomHeaders: Set(d, "http_custom_headers").List(),
	}
	return o
}
func expandLogStreamDatadogSink(d ResourceData) *management.DatadogSink {
	o := &management.DatadogSink{
		DatadogRegion: String(d, "datadog_region"),
		DatadogAPIKey: String(d, "datadog_api_key"),
	}
	return o
}
func expandLogStreamSplunkSink(d ResourceData) *management.SplunkSink {
	o := &management.SplunkSink{
		SplunkDomain: String(d, "splunk_domain"),
		SplunkToken:  String(d, "splunk_token"),
		SplunkPort:   String(d, "splunk_port"),
		SplunkSecure: Bool(d, "splunk_secure"),
	}
	return o
}

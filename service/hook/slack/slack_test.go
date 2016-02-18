package slack

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/stretchr/testify/require"
)

func Test_detectContentType(t *testing.T) {
	t.Log("Proper Content-Type")
	{
		header := http.Header{
			"Content-Type": {"application/x-www-form-urlencoded"},
		}
		contentType, err := detectContentType(header)
		require.NoError(t, err)
		require.Equal(t, "application/x-www-form-urlencoded", contentType)
	}
	t.Log("Missing Content-Type")
	{
		header := http.Header{}
		contentType, err := detectContentType(header)
		require.EqualError(t, err, "Issue with Content-Type Header: No value found in HEADER for the key: Content-Type")
		require.Equal(t, "", contentType)
	}
}

func Test_createMessageModelFromFormRequest(t *testing.T) {
	t.Log("Proper Form content")
	{
		request := http.Request{}
		form := url.Values{}
		form.Add("trigger_word", "the trigger word")
		form.Add("text", "the text")
		request.PostForm = form

		messageModel, err := createMessageModelFromFormRequest(&request)
		require.NoError(t, err)
		require.Equal(t, MessageModel{
			TriggerText: "the trigger word",
			Text:        "the text",
		}, messageModel)
	}

	t.Log("Missing trigger_word")
	{
		request := http.Request{}
		form := url.Values{}
		form.Add("text", "the text")
		request.PostForm = form

		messageModel, err := createMessageModelFromFormRequest(&request)
		require.EqualError(t, err, "Missing required parameter: 'trigger_word'")
		require.Equal(t, MessageModel{}, messageModel)
	}
	t.Log("Missing text")
	{
		request := http.Request{}
		form := url.Values{}
		form.Add("trigger_word", "the trigger word")
		request.PostForm = form

		messageModel, err := createMessageModelFromFormRequest(&request)
		require.EqualError(t, err, "Missing required parameter: 'text'")
		require.Equal(t, MessageModel{}, messageModel)
	}
}

func Test_collectParamsFromPipeSeparatedText(t *testing.T) {
	t.Log("Single item - trimming")
	{
		texts := []string{
			"key: the value",
			"key : the value",
			"key :the value",
			"key :   the value   ",
			"key: the value |",
		}
		for _, aText := range texts {
			collectedParams := collectParamsFromPipeSeparatedText(aText)
			require.Equal(t, map[string]string{"key": "the value"}, collectedParams)
		}
	}

	t.Log("Single item, includes :")
	{
		collectedParams := collectParamsFromPipeSeparatedText("key: the:value")
		require.Equal(t, map[string]string{"key": "the:value"}, collectedParams)
		collectedParams = collectParamsFromPipeSeparatedText("key: the :value")
		require.Equal(t, map[string]string{"key": "the :value"}, collectedParams)
		collectedParams = collectParamsFromPipeSeparatedText("key: the : value")
		require.Equal(t, map[string]string{"key": "the : value"}, collectedParams)
		collectedParams = collectParamsFromPipeSeparatedText("key: the  :  value")
		require.Equal(t, map[string]string{"key": "the  :  value"}, collectedParams)
		collectedParams = collectParamsFromPipeSeparatedText("key    : the : value")
		require.Equal(t, map[string]string{"key": "the : value"}, collectedParams)
	}

	t.Log("Multiple items")
	{
		collectedParams := collectParamsFromPipeSeparatedText("key1: value 1 |   key2 : value 2")
		require.Equal(t, map[string]string{
			"key1": "value 1",
			"key2": "value 2",
		},
			collectedParams)
	}

	t.Log("Multiple items - empty parts")
	{
		collectedParams := collectParamsFromPipeSeparatedText("|key1: value 1 |   key2 : value 2|")
		require.Equal(t, map[string]string{
			"key2": "value 2",
			"key1": "value 1",
		},
			collectedParams)
	}

	t.Log("Multiple items - formatting test")
	{
		collectedParams := collectParamsFromPipeSeparatedText("|key1: value 1 |   key2 : value 2 |key3:value 3")
		require.Equal(t, map[string]string{
			"key1": "value 1",
			"key3": "value 3",
			"key2": "value 2",
		},
			collectedParams)
	}
}

func Test_transformOutgoingWebhookMessage(t *testing.T) {
	t.Log("Should be OK")
	{
		webhookMsg := MessageModel{
			TriggerText: "bitrise:",
			Text:        "bitrise: branch:master",
		}

		hookTransformResult := transformOutgoingWebhookMessage(webhookMsg)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch: "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Should be OK - space between param key&value")
	{
		webhookMsg := MessageModel{
			TriggerText: "bitrise -",
			Text:        "bitrise - branch: master",
		}

		hookTransformResult := transformOutgoingWebhookMessage(webhookMsg)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch: "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Empty parameter component")
	{
		webhookMsg := MessageModel{
			TriggerText: "bitrise -",
			Text:        "bitrise - branch: master | ",
		}

		hookTransformResult := transformOutgoingWebhookMessage(webhookMsg)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch: "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Message parameter")
	{
		webhookMsg := MessageModel{
			TriggerText: "bitrise -",
			Text:        "bitrise - branch: master | message: this is the Commit Message param",
		}

		hookTransformResult := transformOutgoingWebhookMessage(webhookMsg)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:        "master",
					CommitMessage: "this is the Commit Message param",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Commit parameter")
	{
		webhookMsg := MessageModel{
			TriggerText: "bitrise -",
			Text:        "bitrise - branch: master | commit: cmtHash123",
		}

		hookTransformResult := transformOutgoingWebhookMessage(webhookMsg)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:     "master",
					CommitHash: "cmtHash123",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Tag parameter")
	{
		webhookMsg := MessageModel{
			TriggerText: "bitrise -",
			Text:        "bitrise - tag: v1.0|branch : develop",
		}

		hookTransformResult := transformOutgoingWebhookMessage(webhookMsg)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch: "develop",
					Tag:    "v1.0",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("All parameters")
	{
		webhookMsg := MessageModel{
			TriggerText: "bitrise -",
			Text:        "bitrise - branch : develop | tag: v1.1|  message : this is:my message  | commit: cmtHash321",
		}

		hookTransformResult := transformOutgoingWebhookMessage(webhookMsg)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:        "develop",
					Tag:           "v1.1",
					CommitHash:    "cmtHash321",
					CommitMessage: "this is:my message",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Missing branch parameter")
	{
		webhookMsg := MessageModel{
			TriggerText: "bitrise:",
			Text:        "bitrise: no branch",
		}

		hookTransformResult := transformOutgoingWebhookMessage(webhookMsg)
		require.EqualError(t, hookTransformResult.Error, "Missing branch parameter!")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}
}

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	t.Log("Should be OK")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/x-www-form-urlencoded"},
			},
		}
		form := url.Values{}
		form.Add("trigger_word", "bitrise:")
		form.Add("text", "bitrise: branch:master")
		request.PostForm = form

		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch: "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Unsupported Event Type")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Content-Type is not supported: application/json")
	}

	t.Log("Missing 'text' from request data")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/x-www-form-urlencoded"},
			},
		}
		form := url.Values{}
		form.Add("trigger_word", "the trigger word")
		request.PostForm = form

		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to parse the request/message: Missing required parameter: 'text'")
	}
}

func Test_HookProvider_TransformResponse(t *testing.T) {
	provider := HookProvider{}

	t.Log("Single success")
	{
		baseRespModel := hookCommon.TransformResponseInputModel{
			SuccessTriggerResponses: []bitriseapi.TriggerAPIResponseModel{
				{
					Status:    "ok",
					Message:   "triggered build",
					Service:   "bitrise",
					AppSlug:   "app-slug",
					BuildSlug: "build-slug",
				},
			},
		}

		resp := provider.TransformResponse(baseRespModel)
		require.Equal(t, hookCommon.TransformResponseModel{
			Data: OutgoingWebhookRespModel{
				Text: `Results:
*Success!* Details:
* {Status:ok Message:triggered build Service:bitrise AppSlug:app-slug BuildSlug:build-slug}`,
			},
			HTTPStatusCode: 200,
		}, resp)
	}

	t.Log("Single failed trigger")
	{
		baseRespModel := hookCommon.TransformResponseInputModel{
			FailedTriggerResponses: []bitriseapi.TriggerAPIResponseModel{
				{
					Status:    "error",
					Message:   "some error happened",
					Service:   "bitrise",
					AppSlug:   "app-slug",
					BuildSlug: "build-slug",
				},
			},
		}

		resp := provider.TransformResponse(baseRespModel)
		require.Equal(t, hookCommon.TransformResponseModel{
			Data: OutgoingWebhookRespModel{
				Text: `Results:
*[!] Failed Triggers*:
* {Status:error Message:some error happened Service:bitrise AppSlug:app-slug BuildSlug:build-slug}`,
			},
			HTTPStatusCode: 200,
		}, resp)
	}

	t.Log("Single error")
	{
		baseRespModel := hookCommon.TransformResponseInputModel{
			Errors: []string{"a single error"},
		}

		resp := provider.TransformResponse(baseRespModel)
		require.Equal(t, hookCommon.TransformResponseModel{
			Data: OutgoingWebhookRespModel{
				Text: `Results:
*[!] Errors*:
* a single error`,
			},
			HTTPStatusCode: 200,
		}, resp)
	}

	t.Log("Multiple errors")
	{
		baseRespModel := hookCommon.TransformResponseInputModel{
			Errors: []string{"first error", "Second Error"},
		}

		resp := provider.TransformResponse(baseRespModel)
		require.Equal(t, hookCommon.TransformResponseModel{
			Data: OutgoingWebhookRespModel{
				Text: `Results:
*[!] Errors*:
* first error
* Second Error`,
			},
			HTTPStatusCode: 200,
		}, resp)
	}
}

func Test_HookProvider_TransformErrorMessageResponse(t *testing.T) {
	provider := HookProvider{}

	{
		resp := provider.TransformErrorMessageResponse("my Err msg")
		require.Equal(t, hookCommon.TransformResponseModel{
			Data: OutgoingWebhookRespModel{
				Text: "*[!] Error*: my Err msg",
			},
			HTTPStatusCode: 200,
		}, resp)
	}
}

func Test_HookProvider_TransformSuccessMessageResponse(t *testing.T) {
	provider := HookProvider{}

	{
		resp := provider.TransformSuccessMessageResponse("my Success msg")
		require.Equal(t, hookCommon.TransformResponseModel{
			Data: OutgoingWebhookRespModel{
				Text: "my Success msg",
			},
			HTTPStatusCode: 200,
		}, resp)
	}
}
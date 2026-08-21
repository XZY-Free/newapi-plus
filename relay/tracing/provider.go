package tracing

import (
	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

// ProviderName 由 NewAPI Channel Type 集中映射（V1.1 §9.10），
// 不允许各 Adapter 自行填写。Provider 表示真实提供方/系统，协议格式是另一个概念。
// 未列出的 Channel Type 使用稳定小写名称；无法识别的统一回退 "unknown"。
func ProviderName(channelType int) string {
	switch channelType {
	case constant.ChannelTypeOpenAI, constant.ChannelTypeOpenAIMax, constant.ChannelTypeSubmodel,
		constant.ChannelTypeCustom, constant.ChannelTypeSub2API, constant.ChannelTypeAdvancedCustom,
		constant.ChannelTypeNewAPI:
		return "openai"
	case constant.ChannelTypeAzure:
		return "azure.ai.openai"
	case constant.ChannelTypeAnthropic:
		return "anthropic"
	case constant.ChannelTypeAws:
		return "aws.bedrock"
	case constant.ChannelTypeGemini:
		return "gcp.gemini"
	case constant.ChannelTypeVertexAi:
		return "gcp.vertex_ai"
	case constant.ChannelTypeOllama:
		return "ollama"
	case constant.ChannelTypeBaidu, constant.ChannelTypeBaiduV2:
		return "baidu"
	case constant.ChannelTypeZhipu, constant.ChannelTypeZhipu_v4:
		return "zhipu"
	case constant.ChannelTypeAli:
		return "aliyun"
	case constant.ChannelTypeXunfei:
		return "iflytek"
	case constant.ChannelTypeTencent:
		return "tencent"
	case constant.ChannelTypeMoonshot:
		return "moonshot"
	case constant.ChannelTypePerplexity:
		return "perplexity"
	case constant.ChannelTypeLingYiWanWu:
		return "lingyiwanwu"
	case constant.ChannelTypeCohere:
		return "cohere"
	case constant.ChannelTypeMiniMax:
		return "minimax"
	case constant.ChannelTypeJina:
		return "jina"
	case constant.ChannelTypeSiliconFlow:
		return "siliconflow"
	case constant.ChannelTypeMistral:
		return "mistral"
	case constant.ChannelTypeDeepSeek:
		return "deepseek"
	case constant.ChannelTypeMokaAI:
		return "mokiai"
	case constant.ChannelTypeVolcEngine:
		return "volcengine"
	case constant.ChannelTypeXinference:
		return "xinference"
	case constant.ChannelTypeXai:
		return "xai"
	case constant.ChannelTypeCoze:
		return "coze"
	case constant.ChannelTypeKling:
		return "kling"
	case constant.ChannelTypeJimeng:
		return "jimeng"
	case constant.ChannelTypeReplicate:
		return "replicate"
	case constant.ChannelTypeCodex:
		return "codex"
	case constant.ChannelTypeOpenRouter:
		return "openrouter"
	case constant.ChannelTypeMidjourney, constant.ChannelTypeMidjourneyPlus:
		return "midjourney"
	case constant.ChannelTypeSunoAPI:
		return "suno"
	case constant.ChannelTypeDify:
		return "dify"
	case constant.ChannelTypePaLM:
		return "gcp.palm"
	default:
		return "unknown"
	}
}

// OperationName 由 NewAPI RelayMode 映射到 GenAI operation name（V1.1 §9.8）。
// Claude 请求的 RelayMode 即 ChatCompletions；rerank / moderation 作为受控自定义值；
// 检索类 Search 使用标准预定义 retrieval。
func OperationName(relayMode int) string {
	switch relayMode {
	case relayconstant.RelayModeEmbeddings:
		return "embeddings"
	case relayconstant.RelayModeRerank:
		return "rerank"
	case relayconstant.RelayModeAlphaSearch:
		return "retrieval"
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits,
		relayconstant.RelayModeAudioSpeech, relayconstant.RelayModeAudioTranscription,
		relayconstant.RelayModeAudioTranslation:
		return "generate_content"
	case relayconstant.RelayModeChatCompletions, relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		return "chat"
	case relayconstant.RelayModeCompletions:
		return "text_completion"
	case relayconstant.RelayModeModerations:
		return "moderation"
	default:
		return "chat"
	}
}

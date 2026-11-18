package providers

import (
	"fmt"
	"os"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/llm"
	claudedriver "github.com/peggco/pegg/internal/llm/drivers/claude"
	geminidriver "github.com/peggco/pegg/internal/llm/drivers/gemini"
	openaidriver "github.com/peggco/pegg/internal/llm/drivers/openai"
)

type driverKind int

const (
	driverClaude driverKind = iota + 1
	driverGemini
)

type spec struct {
	Name       string
	BaseURL    string
	EnvBaseURL string
	Driver     driverKind
}

var specs = []spec{
	{Name: "302ai", BaseURL: "https://api.302.ai/v1"},
	{Name: "abacus", BaseURL: "https://routellm.abacus.ai/v1"},
	{Name: "abliteration-ai", BaseURL: "https://api.abliteration.ai/v1"},
	{Name: "above", BaseURL: "https://api.above.dev/v1"},
	{Name: "agentrouter", BaseURL: "https://agentrouter.org/v1"},
	{Name: "agnes", BaseURL: "https://apihub.agnes-ai.com/v1"},
	{Name: "ai-router", BaseURL: "https://api.ai-router.dev/v1"},
	{Name: "ai21", BaseURL: "https://api.ai21.com/studio/v1"},
	{Name: "aiand", BaseURL: "https://api.aiand.com/v1"},
	{Name: "aimlapi", BaseURL: "https://api.aimlapi.com/v1"},
	{Name: "ainetcafe", BaseURL: "https://microquickjs.com/v1"},
	{Name: "aixy", BaseURL: "https://api.aixy-gateway.com/v1"},
	{Name: "aki-io", BaseURL: "https://aki.io/v1"},
	{Name: "alibaba", BaseURL: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"},
	{Name: "alibaba-cn", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1"},
	{Name: "alibaba-coding-plan", BaseURL: "https://coding-intl.dashscope.aliyuncs.com/v1"},
	{Name: "alibaba-coding-plan-cn", BaseURL: "https://coding.dashscope.aliyuncs.com/v1"},
	{Name: "alibaba-token-plan", BaseURL: "https://token-plan.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1"},
	{Name: "alibaba-token-plan-cn", BaseURL: "https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1"},
	{Name: "ambient", BaseURL: "https://api.ambient.xyz/v1"},
	{Name: "amd", BaseURL: "https://developer.amd.com.cn/radeon/api/v1"},
	{Name: "anthropic", BaseURL: "https://api.anthropic.com", Driver: driverClaude},
	{Name: "anyapi", BaseURL: "https://api.anyapi.ai/v1"},
	{Name: "anyscale", BaseURL: "https://api.endpoints.anyscale.com/v1"},
	{Name: "arcee", BaseURL: "https://api.arcee.ai/api/v1"},
	{Name: "atomic-chat", BaseURL: "http://127.0.0.1:1337/v1"},
	{Name: "auriko", BaseURL: "https://api.auriko.ai/v1"},
	{Name: "baichuan", BaseURL: "https://api.baichuan-ai.com/v1"},
	{Name: "bailing", BaseURL: "https://api.tbox.cn/api/llm/v1"},
	{Name: "baseten", BaseURL: "https://inference.baseten.co/v1"},
	{Name: "bee", BaseURL: "https://api.bee.heossi.com/bee"},
	{Name: "berget", BaseURL: "https://api.berget.ai/v1"},
	{Name: "blueclaw", BaseURL: "https://openai.blueclaw.network/v1"},
	{Name: "bothub", BaseURL: "https://openai.bothub.ru/v1"},
	{Name: "boundless", BaseURL: "https://api.inference.boundless.network/v1"},
	{Name: "cerebras", BaseURL: "https://api.cerebras.ai/v1"},
	{Name: "chutes", BaseURL: "https://llm.chutes.ai/v1"},
	{Name: "clarifai", BaseURL: "https://api.clarifai.com/v2/ext/openai/v1"},
	{Name: "claudinio", BaseURL: "https://api.claudin.io/v1"},
	{Name: "cloudferro-sherlock", BaseURL: "https://api-sherlock.cloudferro.com/openai/v1/"},
	{Name: "cloudflare-workers-ai", EnvBaseURL: "https://api.cloudflare.com/client/v4/accounts/${CLOUDFLARE_ACCOUNT_ID}/ai/v1"},
	{Name: "cohere", BaseURL: "https://api.cohere.com/compatibility/v1"},
	{Name: "commandcode", BaseURL: "https://api.commandcode.ai/provider/v1"},
	{Name: "coralbricks", BaseURL: "https://inference.coralbricks.ai/v1"},
	{Name: "cortecs", BaseURL: "https://api.cortecs.ai/v1"},
	{Name: "crossmodel", BaseURL: "https://api.crossmodel.ai/v1"},
	{Name: "crusoe", BaseURL: "https://api.inference.crusoecloud.com/v1"},
	{Name: "daoxe", BaseURL: "https://daoxe.com/v1"},
	{Name: "databricks", EnvBaseURL: "https://${DATABRICKS_HOST}/ai-gateway/mlflow/v1"},
	{Name: "deepinfra", BaseURL: "https://api.deepinfra.com/v1/openai"},
	{Name: "deepseek", BaseURL: "https://api.deepseek.com/v1"},
	{Name: "digitalocean", BaseURL: "https://inference.do-ai.run/v1"},
	{Name: "dinference", BaseURL: "https://api.dinference.com/v1"},
	{Name: "drun", BaseURL: "https://chat.d.run/v1"},
	{Name: "ebcloud", BaseURL: "https://maas-api.ebcloud.com/v1"},
	{Name: "echo", BaseURL: "https://echo.tracerml.ai/v1"},
	{Name: "edenai", BaseURL: "https://api.edenai.run/v3"},
	{Name: "empiriolabs", BaseURL: "https://api.empiriolabs.ai/v1"},
	{Name: "engy", BaseURL: "https://api.engy.ai/v1"},
	{Name: "evren", BaseURL: "https://evren-llmapi.ssyz.org.tr/v1"},
	{Name: "evroc", BaseURL: "https://models.think.evroc.com/v1"},
	{Name: "fastrouter", BaseURL: "https://go.fastrouter.ai/api/v1"},
	{Name: "fireworks", BaseURL: "https://api.fireworks.ai/inference/v1"},
	{Name: "fireworks-ai", BaseURL: "https://api.fireworks.ai/inference/v1/"},
	{Name: "freemodel", BaseURL: "https://cc.freemodel.dev", Driver: driverClaude},
	{Name: "friendli", BaseURL: "https://api.friendli.ai/serverless/v1"},
	{Name: "frogbot", BaseURL: "https://app.frogbot.ai/api/v1"},
	{Name: "github-copilot", BaseURL: "https://api.githubcopilot.com"},
	{Name: "gmicloud", BaseURL: "https://api.gmi-serving.com/v1"},
	{Name: "google", BaseURL: "https://generativelanguage.googleapis.com/v1beta", Driver: driverGemini},
	{Name: "greenpt", BaseURL: "https://api.greenpt.ai/v1"},
	{Name: "groq", BaseURL: "https://api.groq.com/openai/v1"},
	{Name: "helicone", BaseURL: "https://ai-gateway.helicone.ai/v1"},
	{Name: "hetzner", BaseURL: "https://inference.hetzner.com/api/v1"},
	{Name: "hopper", BaseURL: "https://api.withhopper.com/v1"},
	{Name: "hpc-ai", BaseURL: "https://api.hpc-ai.com/inference/v1"},
	{Name: "huggingface", BaseURL: "https://router.huggingface.co/v1"},
	{Name: "hyper", BaseURL: "https://hyper.charm.land/v1"},
	{Name: "impossibl", BaseURL: "https://api.impossibl.com/v1"},
	{Name: "inception", BaseURL: "https://api.inceptionlabs.ai/v1/"},
	{Name: "inceptron", BaseURL: "https://api.inceptron.io/v1"},
	{Name: "inco", BaseURL: "https://api.inco.ai/v1"},
	{Name: "infer", BaseURL: "https://infer.flow7.org/v1"},
	{Name: "inference", BaseURL: "https://inference.net/v1"},
	{Name: "inferx", BaseURL: "https://model.inferx.net/endpoints/v1"},
	{Name: "infomaniak", EnvBaseURL: "https://api.infomaniak.com/2/ai/${INFOMANIAK_PRODUCT_ID}/openai/v1"},
	{Name: "io-net", BaseURL: "https://api.intelligence.io.solutions/api/v1"},
	{Name: "iteracompute", BaseURL: "https://api.iteracompute.com/v1"},
	{Name: "jalapeno", BaseURL: "https://api.jalapeno-cloud.ai/v1"},
	{Name: "jiekou", BaseURL: "https://api.jiekou.ai/openai"},
	{Name: "kenari", BaseURL: "https://kenari.id/v1"},
	{Name: "kilo", BaseURL: "https://api.kilo.ai/api/gateway"},
	{Name: "kimi-code-plan-cn", BaseURL: "https://api.kimi.com/coding/v1"},
	{Name: "kimi-code-plan-global", BaseURL: "https://api.kimi.ai/coding/v1"},
	{Name: "klokintegration", BaseURL: "https://api-gw.klok.ipaas.se/proxy/kloker-key/v1"},
	{Name: "kosmik", BaseURL: "https://api.koscompute.com/v1"},
	{Name: "kuae-cloud-coding-plan", BaseURL: "https://coding-plan-endpoint.kuaecloud.net/v1"},
	{Name: "lepton", BaseURL: "https://api.lepton.ai/v1"},
	{Name: "lilac", BaseURL: "https://api.getlilac.com/v1"},
	{Name: "liquid", BaseURL: "https://router.inference.ai.exchange/v1"},
	{Name: "llmgateway", BaseURL: "https://api.llmgateway.io/v1"},
	{Name: "llmgateway-providers", BaseURL: "https://api.llmgateway.io/v1"},
	{Name: "llmtech", BaseURL: "https://api.llmtech.eu/v1"},
	{Name: "llmtr", BaseURL: "https://llmtr.com/v1"},
	{Name: "lmstudio", BaseURL: "http://127.0.0.1:1234/v1"},
	{Name: "longcat", BaseURL: "https://api.longcat.chat/openai"},
	{Name: "lowrouter", BaseURL: "https://api.lowrouter.ai/v1"},
	{Name: "lucidquery", BaseURL: "https://api.lucidquery.com/v1"},
	{Name: "lynkr", BaseURL: "http://127.0.0.1:8081/v1"},
	{Name: "meganova", BaseURL: "https://api.meganova.ai/v1"},
	{Name: "melious", BaseURL: "https://api.melious.ai/v1"},
	{Name: "meta", BaseURL: "https://api.meta.ai/v1"},
	{Name: "minimax", BaseURL: "https://api.minimax.chat/v1"},
	{Name: "minimax-cn", BaseURL: "https://api.minimax.cn/anthropic", Driver: driverClaude},
	{Name: "minimax-cn-coding-plan", BaseURL: "https://api.minimax.cn/anthropic", Driver: driverClaude},
	{Name: "minimax-coding-plan", BaseURL: "https://api.minimax.io/anthropic", Driver: driverClaude},
	{Name: "mistral", BaseURL: "https://api.mistral.ai/v1"},
	{Name: "mixlayer", BaseURL: "https://models.mixlayer.ai/v1"},
	{Name: "moark", BaseURL: "https://moark.com/v1"},
	{Name: "modal", BaseURL: "https://inference.us-west.modal.direct/v1"},
	{Name: "model-oracle-ai", BaseURL: "https://api.modeloracle.com/api/v1"},
	{Name: "modelis", BaseURL: "https://modelishub.com/v1"},
	{Name: "modelscope", BaseURL: "https://api-inference.modelscope.cn/v1"},
	{Name: "modular", BaseURL: "https://api.modular.com/v1"},
	{Name: "moonshot", BaseURL: "https://api.moonshot.cn/v1"},
	{Name: "moonshotai", BaseURL: "https://api.moonshot.ai/v1"},
	{Name: "moonshotai-cn", BaseURL: "https://api.moonshot.cn/v1"},
	{Name: "morph", BaseURL: "https://api.morphllm.com/v1"},
	{Name: "munito", BaseURL: "https://api.munito.ai/inference/v1"},
	{Name: "munito-eu", BaseURL: "https://api.eu.munito.ai/inference/v1"},
	{Name: "nan", BaseURL: "https://api.nan.builders/v1"},
	{Name: "nano-gpt", BaseURL: "https://nano-gpt.com/api/v1"},
	{Name: "nearai", BaseURL: "https://cloud-api.near.ai/v1"},
	{Name: "nebius", BaseURL: "https://api.tokenfactory.nebius.com/v1"},
	{Name: "neon", EnvBaseURL: "${NEON_AI_GATEWAY_BASE_URL}/v1"},
	{Name: "neosmith", BaseURL: "https://router.neosmith.ai/v1"},
	{Name: "neuralwatt", BaseURL: "https://api.neuralwatt.com/v1"},
	{Name: "nova", BaseURL: "https://api.nova.amazon.com/v1"},
	{Name: "novita", BaseURL: "https://api.novita.ai/v3/openai"},
	{Name: "novita-ai", BaseURL: "https://api.novita.ai/openai"},
	{Name: "nvidia", BaseURL: "https://integrate.api.nvidia.com/v1"},
	{Name: "oci", BaseURL: "https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/openai/v1"},
	{Name: "ofox", BaseURL: "https://api.ofox.ai/v1"},
	{Name: "ollama-cloud", BaseURL: "https://ollama.com/v1"},
	{Name: "openai", BaseURL: "https://api.openai.com/v1"},
	{Name: "opencode", BaseURL: "https://opencode.ai/inference/openai/v1"},
	{Name: "openreason", BaseURL: "https://api.openreason.app/v1"},
	{Name: "openrouter", BaseURL: "https://openrouter.ai/api/v1"},
	{Name: "opper", BaseURL: "https://api.opper.ai/v3/compat"},
	{Name: "orcarouter", BaseURL: "https://api.orcarouter.ai/v1"},
	{Name: "ovhcloud", BaseURL: "https://oai.endpoints.kepler.ai.cloud.ovh.net/v1"},
	{Name: "pareto", BaseURL: "https://api.paretoinference.com/v1"},
	{Name: "pendra", BaseURL: "https://api.pendra.ai/api/v1"},
	{Name: "perplexity", BaseURL: "https://api.perplexity.ai"},
	{Name: "perplexity-agent", BaseURL: "https://api.perplexity.ai/v1"},
	{Name: "pioneer", BaseURL: "https://api.fastino.ai/v1"},
	{Name: "poe", BaseURL: "https://api.poe.com/v1"},
	{Name: "poolside", BaseURL: "https://inference.poolside.ai/v1"},
	{Name: "prime", BaseURL: "https://api.pinference.ai/api/v1"},
	{Name: "privatemode-ai", BaseURL: "http://localhost:8080/v1"},
	{Name: "qihang-ai", BaseURL: "https://api.qhaigc.net/v1"},
	{Name: "qiniu-ai", BaseURL: "https://api.qnaigc.com/v1"},
	{Name: "reflection", BaseURL: "https://api.reflection.ai/openai/v1"},
	{Name: "regolo-ai", BaseURL: "https://api.regolo.ai/v1"},
	{Name: "requesty", BaseURL: "https://router.requesty.ai/v1"},
	{Name: "routing-run", BaseURL: "https://api.routing.run/v1"},
	{Name: "runinfra", BaseURL: "https://api.runinfra.ai/v1"},
	{Name: "sakana", BaseURL: "https://api.sakana.ai/v1"},
	{Name: "sambanova", BaseURL: "https://api.sambanova.ai/v1"},
	{Name: "sarvam", BaseURL: "https://api.sarvam.ai/v1"},
	{Name: "scaleway", BaseURL: "https://api.scaleway.ai/v1"},
	{Name: "scnet-token-plan", BaseURL: "https://api.scnet.cn/api/llm/v1"},
	{Name: "scx-ai", BaseURL: "https://api.scx.ai/v1"},
	{Name: "sensenova", BaseURL: "https://token.sensenova.cn/v1"},
	{Name: "siliconflow", BaseURL: "https://api.siliconflow.com/v1"},
	{Name: "siliconflow-cn", BaseURL: "https://api.siliconflow.cn/v1"},
	{Name: "snowflake-cortex", EnvBaseURL: "https://${SNOWFLAKE_ACCOUNT}.snowflakecomputing.com/api/v2/cortex/v1"},
	{Name: "stackit", BaseURL: "https://api.openai-compat.model-serving.eu01.onstackit.cloud/v1"},
	{Name: "standardcompute", BaseURL: "https://api.stdcmpt.com/v1"},
	{Name: "stepfun", BaseURL: "https://api.stepfun.com/v1"},
	{Name: "stepfun-ai", BaseURL: "https://api.stepfun.ai/v1"},
	{Name: "stepfun-ai-step-plan", BaseURL: "https://api.stepfun.ai/step_plan/v1"},
	{Name: "stepfun-step-plan", BaseURL: "https://api.stepfun.com/step_plan/v1"},
	{Name: "subconscious", BaseURL: "https://api.subconscious.dev", Driver: driverClaude},
	{Name: "submodel", BaseURL: "https://llm.submodel.ai/v1"},
	{Name: "synthetic", BaseURL: "https://api.synthetic.new/openai/v1"},
	{Name: "tempr", BaseURL: "https://api.temprhq.io/v1"},
	{Name: "tencent", BaseURL: "https://hunyuan.cloud.tencent.com/v1"},
	{Name: "tencent-coding-plan", BaseURL: "https://api.lkeap.cloud.tencent.com/coding/v3"},
	{Name: "tencent-token-plan", BaseURL: "https://api.lkeap.cloud.tencent.com/plan/v3"},
	{Name: "tencent-tokenhub", BaseURL: "https://tokenhub.tencentmaas.com/v1"},
	{Name: "tensorx", BaseURL: "https://api.tensorx.ai/v1"},
	{Name: "the-grid-ai", BaseURL: "https://api.thegrid.ai/v1"},
	{Name: "thinkingmachines", BaseURL: "https://tinker.thinkingmachines.dev/services/tinker-prod/anthropic/api", Driver: driverClaude},
	{Name: "tinfoil", BaseURL: "https://inference.tinfoil.sh/v1"},
	{Name: "together", BaseURL: "https://api.together.xyz/v1"},
	{Name: "tokengo", BaseURL: "https://api.tokengo.com/v1"},
	{Name: "tokenrouter", BaseURL: "https://api.tokenrouter.com/v1"},
	{Name: "trustedrouter", BaseURL: "https://api.trustedrouter.com/v1"},
	{Name: "umans-ai", BaseURL: "https://api.code.umans.ai/v1"},
	{Name: "umans-ai-coding-plan", BaseURL: "https://api.code.umans.ai/v1"},
	{Name: "unorouter", BaseURL: "https://api.unorouter.com/v1"},
	{Name: "upstage", BaseURL: "https://api.upstage.ai/v1/solar"},
	{Name: "vancine", BaseURL: "https://vancine.com/v1"},
	{Name: "vispark", BaseURL: "https://api.lab.vispark.in/v1"},
	{Name: "vivgrid", BaseURL: "https://api.vivgrid.com/v1"},
	{Name: "volcengine", BaseURL: "https://ark.cn-beijing.volces.com/api/v3"},
	{Name: "volcengine-coding-plan", BaseURL: "https://ark.cn-beijing.volces.com/api/coding/v3"},
	{Name: "voyage", BaseURL: "https://api.voyageai.com/v1"},
	{Name: "vultr", BaseURL: "https://api.vultrinference.com/v1"},
	{Name: "wafer.ai", BaseURL: "https://pass.wafer.ai/v1"},
	{Name: "wallaby", BaseURL: "https://api.wallabytoken.com/v1"},
	{Name: "wally", BaseURL: "https://inference.runanywhere.ai/v1"},
	{Name: "wandb", BaseURL: "https://api.inference.wandb.ai/v1"},
	{Name: "writer", BaseURL: "https://api.writer.com/v1"},
	{Name: "xai", BaseURL: "https://api.x.ai/v1"},
	{Name: "xiaomi", BaseURL: "https://api.xiaomimimo.com/v1"},
	{Name: "xiaomi-token-plan-ams", BaseURL: "https://token-plan-ams.xiaomimimo.com/v1"},
	{Name: "xiaomi-token-plan-cn", BaseURL: "https://token-plan-cn.xiaomimimo.com/v1"},
	{Name: "xiaomi-token-plan-sgp", BaseURL: "https://token-plan-sgp.xiaomimimo.com/v1"},
	{Name: "xpersona", BaseURL: "https://www.xpersona.co/v1"},
	{Name: "yi", BaseURL: "https://api.lingyiwanwu.com/v1"},
	{Name: "zai", BaseURL: "https://api.z.ai/api/paas/v4"},
	{Name: "zai-coding-plan", BaseURL: "https://api.z.ai/api/coding/paas/v4"},
	{Name: "zeldoc", BaseURL: "https://api.zeldoc.ai/v1"},
	{Name: "zenifra", BaseURL: "https://ai.zenifra.com/v1"},
	{Name: "zenmux", BaseURL: "https://zenmux.ai/api/v1"},
	{Name: "zhipu", BaseURL: "https://open.bigmodel.cn/api/paas/v4"},
	{Name: "zhipuai", BaseURL: "https://open.bigmodel.cn/api/paas/v4"},
	{Name: "zhipuai-coding-plan", BaseURL: "https://open.bigmodel.cn/api/coding/paas/v4"},
}

func init() {
	for _, s := range specs {
		llm.RegisterProvider(s.Name, func(cfg config.LLMConfig) (llm.Provider, error) {
			return s.newProvider(cfg)
		})
	}
}

func (s spec) baseURL() (string, error) {
	if s.EnvBaseURL == "" {
		return s.BaseURL, nil
	}

	var missing string
	baseURL := os.Expand(s.EnvBaseURL, func(key string) string {
		v := os.Getenv(key)
		if v == "" && missing == "" {
			missing = key
		}
		return v
	})
	if missing != "" {
		return "", fmt.Errorf("%s: missing environment variable %s", s.Name, missing)
	}
	return baseURL, nil
}

func (s spec) newProvider(cfg config.LLMConfig) (llm.Provider, error) {
	baseURL, err := s.baseURL()
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(cfg.Timeout) * time.Second
	switch s.Driver {
	case driverClaude:
		return claudedriver.NewService(s.Name, claudedriver.ServiceConfig{
			BaseURL: baseURL,
			APIKey:  cfg.APIKey,
			Headers: cfg.Headers,
			Timeout: timeout,
		}), nil
	case driverGemini:
		return geminidriver.NewService(s.Name, geminidriver.ServiceConfig{
			BaseURL: baseURL,
			APIKey:  cfg.APIKey,
			Headers: cfg.Headers,
			Timeout: timeout,
		}), nil
	default:
		return openaidriver.NewService(s.Name, openaidriver.ServiceConfig{
			BaseURL: baseURL,
			APIKey:  cfg.APIKey,
			Headers: cfg.Headers,
			Timeout: timeout,
		}), nil
	}
}

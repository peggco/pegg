---
icon: lucide/boxes
---

# Provider catalog

Pegg ships 236 providers out of the box. Each one is a dedicated package
under `internal/llm/providers/`. Credentials are supplied per provider in
your config (`api_key`). The API key env column lists the conventional
variable name used by the upstream SDK, for reference.

Base URLs containing `${VAR}` are resolved from the environment when the
provider is configured; Pegg reports an error if the variable is unset.

The `claude`, `codex`, `cline`, and `cline-pass` providers use subscriptions
instead of API keys; see [Subscriptions](providers-and-models.md#subscriptions).

Providers that require a dedicated, non-compatible SDK (for example Amazon
Bedrock, Azure OpenAI, Google Vertex, GitLab, watsonx, SAP AI Core, and
Vercel) are not part of this catalog.

| Provider | Driver | Base URL | API key env |
|---|---|---|---|
| `302ai` | openai | `https://api.302.ai/v1` | `302AI_API_KEY` |
| `abacus` | openai | `https://routellm.abacus.ai/v1` | `ABACUS_API_KEY` |
| `abliteration-ai` | openai | `https://api.abliteration.ai/v1` | `ABLIT_KEY` |
| `above` | openai | `https://api.above.dev/v1` | `ABOVE_API_KEY` |
| `agentrouter` | openai | `https://agentrouter.org/v1` | `AGENTROUTER_API_KEY` |
| `agnes` | openai | `https://apihub.agnes-ai.com/v1` | `AGNES_API_KEY` |
| `ai-router` | openai | `https://api.ai-router.dev/v1` | `AI_ROUTER_API_KEY` |
| `ai21` | openai | `https://api.ai21.com/studio/v1` | — |
| `aiand` | openai | `https://api.aiand.com/v1` | `AIAND_API_KEY` |
| `aimlapi` | openai | `https://api.aimlapi.com/v1` | `AIMLAPI_API_KEY` |
| `ainetcafe` | openai | `https://microquickjs.com/v1` | `AINETCAFE_API_KEY` |
| `aixy` | openai | `https://api.aixy-gateway.com/v1` | `AIXY_API_KEY` |
| `aki-io` | openai | `https://aki.io/v1` | `AKI_IO_API_KEY` |
| `alibaba` | openai | `https://dashscope-intl.aliyuncs.com/compatible-mode/v1` | `DASHSCOPE_API_KEY` |
| `alibaba-cn` | openai | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `DASHSCOPE_API_KEY` |
| `alibaba-coding-plan` | openai | `https://coding-intl.dashscope.aliyuncs.com/v1` | `ALIBABA_CODING_PLAN_API_KEY` |
| `alibaba-coding-plan-cn` | openai | `https://coding.dashscope.aliyuncs.com/v1` | `ALIBABA_CODING_PLAN_API_KEY` |
| `alibaba-token-plan` | openai | `https://token-plan.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1` | `ALIBABA_TOKEN_PLAN_API_KEY` |
| `alibaba-token-plan-cn` | openai | `https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1` | `ALIBABA_TOKEN_PLAN_API_KEY` |
| `ambient` | openai | `https://api.ambient.xyz/v1` | `AMBIENT_API_KEY` |
| `amd` | openai | `https://developer.amd.com.cn/radeon/api/v1` | `AMD_API_KEY` |
| `anthropic` | claude | `https://api.anthropic.com` | — |
| `anyapi` | openai | `https://api.anyapi.ai/v1` | `ANYAPI_API_KEY` |
| `anyscale` | openai | `https://api.endpoints.anyscale.com/v1` | — |
| `arcee` | openai | `https://api.arcee.ai/api/v1` | `ARCEE_API_KEY` |
| `atomic-chat` | openai | `http://127.0.0.1:1337/v1` | `ATOMIC_CHAT_API_KEY` |
| `auriko` | openai | `https://api.auriko.ai/v1` | `AURIKO_API_KEY` |
| `baichuan` | openai | `https://api.baichuan-ai.com/v1` | — |
| `bailing` | openai | `https://api.tbox.cn/api/llm/v1` | `BAILING_API_TOKEN` |
| `baseten` | openai | `https://inference.baseten.co/v1` | `BASETEN_API_KEY` |
| `bee` | openai | `https://api.bee.heossi.com/bee` | `BEE_API_KEY` |
| `berget` | openai | `https://api.berget.ai/v1` | `BERGET_API_KEY` |
| `blueclaw` | openai | `https://openai.blueclaw.network/v1` | `BLUECLAW_API_KEY` |
| `bothub` | openai | `https://openai.bothub.ru/v1` | `BOTHUB_API_KEY` |
| `boundless` | openai | `https://api.inference.boundless.network/v1` | `BOUNDLESS_API_KEY` |
| `cerebras` | openai | `https://api.cerebras.ai/v1` | — |
| `chutes` | openai | `https://llm.chutes.ai/v1` | `CHUTES_API_KEY` |
| `clarifai` | openai | `https://api.clarifai.com/v2/ext/openai/v1` | `CLARIFAI_PAT` |
| `claude` | claude | `https://api.anthropic.com` | — |
| `claudinio` | openai | `https://api.claudin.io/v1` | `CLAUDINIO_API_KEY` |
| `cline` | openai | `https://api.cline.bot/api/v1` | — |
| `cline-pass` | openai | `https://api.cline.bot/api/v1` | — |
| `cloudferro-sherlock` | openai | `https://api-sherlock.cloudferro.com/openai/v1/` | `CLOUDFERRO_SHERLOCK_API_KEY` |
| `cloudflare-workers-ai` | openai | `https://api.cloudflare.com/client/v4/accounts/${CLOUDFLARE_ACCOUNT_ID}/ai/v1` | `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_API_KEY` |
| `codex` | codex | `https://chatgpt.com/backend-api/codex` | — |
| `cohere` | openai | `https://api.cohere.com/compatibility/v1` | — |
| `commandcode` | openai | `https://api.commandcode.ai/provider/v1` | `CMD_API_KEY` |
| `coralbricks` | openai | `https://inference.coralbricks.ai/v1` | `CORAL_API_KEY` |
| `cortecs` | openai | `https://api.cortecs.ai/v1` | `CORTECS_API_KEY` |
| `crossmodel` | openai | `https://api.crossmodel.ai/v1` | `CROSSMODEL_API_KEY` |
| `crusoe` | openai | `https://api.inference.crusoecloud.com/v1` | `CRUSOE_API_KEY` |
| `daoxe` | openai | `https://daoxe.com/v1` | `DAOXE_API_KEY` |
| `databricks` | openai | `https://${DATABRICKS_HOST}/ai-gateway/mlflow/v1` | `DATABRICKS_HOST`, `DATABRICKS_TOKEN` |
| `deepinfra` | openai | `https://api.deepinfra.com/v1/openai` | — |
| `deepseek` | openai | `https://api.deepseek.com/v1` | — |
| `digitalocean` | openai | `https://inference.do-ai.run/v1` | `DIGITALOCEAN_ACCESS_TOKEN` |
| `dinference` | openai | `https://api.dinference.com/v1` | `DINFERENCE_API_KEY` |
| `drun` | openai | `https://chat.d.run/v1` | `DRUN_API_KEY` |
| `ebcloud` | openai | `https://maas-api.ebcloud.com/v1` | `EBCLOUD_API_KEY` |
| `echo` | openai | `https://echo.tracerml.ai/v1` | `ECHO_API_KEY` |
| `edenai` | openai | `https://api.edenai.run/v3` | `EDENAI_API_KEY` |
| `empiriolabs` | openai | `https://api.empiriolabs.ai/v1` | `EMPIRIOLABS_API_KEY` |
| `engy` | openai | `https://api.engy.ai/v1` | `ENGY_API_KEY` |
| `evren` | openai | `https://evren-llmapi.ssyz.org.tr/v1` | `EVREN_API_KEY` |
| `evroc` | openai | `https://models.think.evroc.com/v1` | `EVROC_API_KEY` |
| `fastrouter` | openai | `https://go.fastrouter.ai/api/v1` | `FASTROUTER_API_KEY` |
| `fireworks` | openai | `https://api.fireworks.ai/inference/v1` | — |
| `fireworks-ai` | openai | `https://api.fireworks.ai/inference/v1/` | `FIREWORKS_API_KEY` |
| `freemodel` | claude | `https://cc.freemodel.dev/v1` | `FREEMODEL_API_KEY` |
| `friendli` | openai | `https://api.friendli.ai/serverless/v1` | `FRIENDLI_TOKEN` |
| `frogbot` | openai | `https://app.frogbot.ai/api/v1` | `FROGBOT_API_KEY` |
| `github-copilot` | openai | `https://api.githubcopilot.com` | `GITHUB_TOKEN` |
| `gmicloud` | openai | `https://api.gmi-serving.com/v1` | `GMICLOUD_API_KEY` |
| `google` | gemini | `https://generativelanguage.googleapis.com/v1beta` | — |
| `greenpt` | openai | `https://api.greenpt.ai/v1` | `GREENPT_API_KEY` |
| `groq` | openai | `https://api.groq.com/openai/v1` | — |
| `helicone` | openai | `https://ai-gateway.helicone.ai/v1` | `HELICONE_API_KEY` |
| `hetzner` | openai | `https://inference.hetzner.com/api/v1` | `HETZNER_API_KEY` |
| `hopper` | openai | `https://api.withhopper.com/v1` | `HOPPER_API_KEY` |
| `hpc-ai` | openai | `https://api.hpc-ai.com/inference/v1` | `HPC_AI_API_KEY` |
| `huggingface` | openai | `https://router.huggingface.co/v1` | `HF_TOKEN` |
| `hyper` | openai | `https://hyper.charm.land/v1` | `HYPER_API_KEY` |
| `impossibl` | openai | `https://api.impossibl.com/v1` | `IMPOSSIBL_API_KEY` |
| `inception` | openai | `https://api.inceptionlabs.ai/v1/` | `INCEPTION_API_KEY` |
| `inceptron` | openai | `https://api.inceptron.io/v1` | `INCEPTRON_API_KEY` |
| `inco` | openai | `https://api.inco.ai/v1` | `INCO_API_KEY` |
| `infer` | openai | `https://infer.flow7.org/v1` | `INFER_API_KEY` |
| `inference` | openai | `https://inference.net/v1` | `INFERENCE_API_KEY` |
| `inferx` | openai | `https://model.inferx.net/endpoints/v1` | `INFERX_API_KEY` |
| `infomaniak` | openai | `https://api.infomaniak.com/2/ai/${INFOMANIAK_PRODUCT_ID}/openai/v1` | `INFOMANIAK_API_KEY`, `INFOMANIAK_PRODUCT_ID` |
| `io-net` | openai | `https://api.intelligence.io.solutions/api/v1` | `IOINTELLIGENCE_API_KEY` |
| `iteracompute` | openai | `https://api.iteracompute.com/v1` | `ITERACOMPUTE_API_KEY` |
| `jalapeno` | openai | `https://api.jalapeno-cloud.ai/v1` | `JALAPENO_API_KEY` |
| `jiekou` | openai | `https://api.jiekou.ai/openai` | `JIEKOU_API_KEY` |
| `kenari` | openai | `https://kenari.id/v1` | `KENARI_API_KEY` |
| `kilo` | openai | `https://api.kilo.ai/api/gateway` | `KILO_API_KEY` |
| `kimi-code-plan-cn` | openai | `https://api.kimi.com/coding/v1` | `KIMI_API_KEY` |
| `kimi-code-plan-global` | openai | `https://api.kimi.ai/coding/v1` | `KIMI_API_KEY` |
| `klokintegration` | openai | `https://api-gw.klok.ipaas.se/proxy/kloker-key/v1` | `KLOKINTEGRATION_API_KEY` |
| `kosmik` | openai | `https://api.koscompute.com/v1` | `KOSMIK_API_KEY` |
| `kuae-cloud-coding-plan` | openai | `https://coding-plan-endpoint.kuaecloud.net/v1` | `KUAE_API_KEY` |
| `lepton` | openai | `https://api.lepton.ai/v1` | — |
| `lilac` | openai | `https://api.getlilac.com/v1` | `LILAC_API_KEY` |
| `liquid` | openai | `https://router.inference.ai.exchange/v1` | `LIQUID_API_KEY` |
| `llmgateway` | openai | `https://api.llmgateway.io/v1` | `LLMGATEWAY_API_KEY` |
| `llmgateway-providers` | openai | `https://api.llmgateway.io/v1` | `LLMGATEWAY_API_KEY` |
| `llmtech` | openai | `https://api.llmtech.eu/v1` | `LLMTECH_API_KEY` |
| `llmtr` | openai | `https://llmtr.com/v1` | `LLMTR_API_KEY` |
| `lmstudio` | openai | `http://127.0.0.1:1234/v1` | `LMSTUDIO_API_KEY` |
| `longcat` | openai | `https://api.longcat.chat/openai` | `LONGCAT_API_KEY` |
| `lowrouter` | openai | `https://api.lowrouter.ai/v1` | `LOWROUTER_API_KEY` |
| `lucidquery` | openai | `https://api.lucidquery.com/v1` | `LUCIDQUERY_API_KEY` |
| `lynkr` | openai | `http://127.0.0.1:8081/v1` | `LYNKR_API_KEY` |
| `meganova` | openai | `https://api.meganova.ai/v1` | `MEGANOVA_API_KEY` |
| `melious` | openai | `https://api.melious.ai/v1` | `MELIOUS_API_KEY` |
| `meta` | openai | `https://api.meta.ai/v1` | `META_MODEL_API_KEY` |
| `minimax` | openai | `https://api.minimax.chat/v1` | — |
| `minimax-cn` | claude | `https://api.minimax.cn/anthropic/v1` | `MINIMAX_API_KEY` |
| `minimax-cn-coding-plan` | claude | `https://api.minimax.cn/anthropic/v1` | `MINIMAX_API_KEY` |
| `minimax-coding-plan` | claude | `https://api.minimax.io/anthropic/v1` | `MINIMAX_API_KEY` |
| `mistral` | openai | `https://api.mistral.ai/v1` | — |
| `mixlayer` | openai | `https://models.mixlayer.ai/v1` | `MIXLAYER_API_KEY` |
| `moark` | openai | `https://moark.com/v1` | `MOARK_API_KEY` |
| `modal` | openai | `https://inference.us-west.modal.direct/v1` | `MODAL_PROXY_TOKEN` |
| `model-oracle-ai` | openai | `https://api.modeloracle.com/api/v1` | `MODEL_ORACLE_API_KEY` |
| `modelis` | openai | `https://modelishub.com/v1` | `MODELIS_API_KEY` |
| `modelscope` | openai | `https://api-inference.modelscope.cn/v1` | `MODELSCOPE_API_KEY` |
| `modular` | openai | `https://api.modular.com/v1` | `MODULAR_API_KEY` |
| `moonshot` | openai | `https://api.moonshot.cn/v1` | — |
| `moonshotai` | openai | `https://api.moonshot.ai/v1` | `MOONSHOT_API_KEY` |
| `moonshotai-cn` | openai | `https://api.moonshot.cn/v1` | `MOONSHOT_API_KEY` |
| `morph` | openai | `https://api.morphllm.com/v1` | `MORPH_API_KEY` |
| `munito` | openai | `https://api.munito.ai/inference/v1` | `MUNITO_API_KEY` |
| `munito-eu` | openai | `https://api.eu.munito.ai/inference/v1` | `MUNITO_API_KEY` |
| `nan` | openai | `https://api.nan.builders/v1` | `NAN_API_KEY` |
| `nano-gpt` | openai | `https://nano-gpt.com/api/v1` | `NANO_GPT_API_KEY` |
| `nearai` | openai | `https://cloud-api.near.ai/v1` | `NEARAI_API_KEY` |
| `nebius` | openai | `https://api.tokenfactory.nebius.com/v1` | `NEBIUS_API_KEY` |
| `neon` | openai | `${NEON_AI_GATEWAY_BASE_URL}/v1` | `NEON_AI_GATEWAY_BASE_URL`, `NEON_AI_GATEWAY_TOKEN` |
| `neosmith` | openai | `https://router.neosmith.ai/v1` | `NEOSMITH_API_KEY` |
| `neuralwatt` | openai | `https://api.neuralwatt.com/v1` | `NEURALWATT_API_KEY` |
| `nova` | openai | `https://api.nova.amazon.com/v1` | `NOVA_API_KEY` |
| `novita` | openai | `https://api.novita.ai/v3/openai` | — |
| `novita-ai` | openai | `https://api.novita.ai/openai` | `NOVITA_API_KEY` |
| `nvidia` | openai | `https://integrate.api.nvidia.com/v1` | — |
| `oci` | openai | `https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/openai/v1` | `OCI_GENAI_API_KEY` |
| `ofox` | openai | `https://api.ofox.ai/v1` | `OFOX_API_KEY` |
| `ollama-cloud` | openai | `https://ollama.com/v1` | `OLLAMA_API_KEY` |
| `openai` | openai | `https://api.openai.com/v1` | — |
| `opencode` | openai | `https://opencode.ai/inference/openai/v1` | `OPENCODE_API_KEY` |
| `opencode-go` | openai | `https://opencode.ai/zen/go/v1` | — |
| `opencode-zen` | openai | `https://opencode.ai/zen/v1` | — |
| `openreason` | openai | `https://api.openreason.app/v1` | `OPENREASON_API_KEY` |
| `openrouter` | openai | `https://openrouter.ai/api/v1` | — |
| `opper` | openai | `https://api.opper.ai/v3/compat` | `OPPER_API_KEY` |
| `orcarouter` | openai | `https://api.orcarouter.ai/v1` | `ORCAROUTER_API_KEY` |
| `ovhcloud` | openai | `https://oai.endpoints.kepler.ai.cloud.ovh.net/v1` | `OVHCLOUD_API_KEY` |
| `pareto` | openai | `https://api.paretoinference.com/v1` | `PARETO_API_KEY` |
| `pendra` | openai | `https://api.pendra.ai/api/v1` | `PENDRA_API_KEY` |
| `perplexity` | openai | `https://api.perplexity.ai` | — |
| `perplexity-agent` | openai | `https://api.perplexity.ai/v1` | `PERPLEXITY_API_KEY` |
| `pioneer` | openai | `https://api.fastino.ai/v1` | `PIONEER_API_KEY` |
| `poe` | openai | `https://api.poe.com/v1` | `POE_API_KEY` |
| `poolside` | openai | `https://inference.poolside.ai/v1` | `POOLSIDE_API_KEY` |
| `prime` | openai | `https://api.pinference.ai/api/v1` | `PRIME_API_KEY` |
| `privatemode-ai` | openai | `http://localhost:8080/v1` | `PRIVATEMODE_API_KEY` |
| `qihang-ai` | openai | `https://api.qhaigc.net/v1` | `QIHANG_API_KEY` |
| `qiniu-ai` | openai | `https://api.qnaigc.com/v1` | `QINIU_API_KEY` |
| `reflection` | openai | `https://api.reflection.ai/openai/v1` | `REFLECTION_API_KEY` |
| `regolo-ai` | openai | `https://api.regolo.ai/v1` | `REGOLO_API_KEY` |
| `requesty` | openai | `https://router.requesty.ai/v1` | `REQUESTY_API_KEY` |
| `routing-run` | openai | `https://api.routing.run/v1` | `ROUTING_RUN_API_KEY` |
| `runinfra` | openai | `https://api.runinfra.ai/v1` | `RUNINFRA_GATEWAY_KEY` |
| `sakana` | openai | `https://api.sakana.ai/v1` | `SAKANA_API_KEY` |
| `sambanova` | openai | `https://api.sambanova.ai/v1` | — |
| `sarvam` | openai | `https://api.sarvam.ai/v1` | `SARVAM_API_KEY` |
| `scaleway` | openai | `https://api.scaleway.ai/v1` | `SCALEWAY_API_KEY` |
| `scnet-token-plan` | openai | `https://api.scnet.cn/api/llm/v1` | `SCNET_API_KEY` |
| `scx-ai` | openai | `https://api.scx.ai/v1` | `SCX_API_KEY` |
| `sensenova` | openai | `https://token.sensenova.cn/v1` | `SENSENOVA_API_KEY` |
| `siliconflow` | openai | `https://api.siliconflow.com/v1` | `SILICONFLOW_API_KEY` |
| `siliconflow-cn` | openai | `https://api.siliconflow.cn/v1` | `SILICONFLOW_CN_API_KEY` |
| `snowflake-cortex` | openai | `https://${SNOWFLAKE_ACCOUNT}.snowflakecomputing.com/api/v2/cortex/v1` | `SNOWFLAKE_ACCOUNT`, `SNOWFLAKE_CORTEX_PAT` |
| `stackit` | openai | `https://api.openai-compat.model-serving.eu01.onstackit.cloud/v1` | `STACKIT_API_KEY` |
| `standardcompute` | openai | `https://api.stdcmpt.com/v1` | `STANDARDCOMPUTE_API_KEY` |
| `stepfun` | openai | `https://api.stepfun.com/v1` | — |
| `stepfun-ai` | openai | `https://api.stepfun.ai/v1` | `STEPFUN_API_KEY` |
| `stepfun-ai-step-plan` | openai | `https://api.stepfun.ai/step_plan/v1` | `STEPFUN_API_KEY` |
| `stepfun-step-plan` | openai | `https://api.stepfun.com/step_plan/v1` | `STEPFUN_API_KEY` |
| `subconscious` | claude | `https://api.subconscious.dev/v1` | `SUBCONSCIOUS_API_KEY` |
| `submodel` | openai | `https://llm.submodel.ai/v1` | `SUBMODEL_INSTAGEN_ACCESS_KEY` |
| `synthetic` | openai | `https://api.synthetic.new/openai/v1` | `SYNTHETIC_API_KEY` |
| `tempr` | openai | `https://api.temprhq.io/v1` | `TEMPR_API_KEY` |
| `tencent` | openai | `https://hunyuan.cloud.tencent.com/v1` | — |
| `tencent-coding-plan` | openai | `https://api.lkeap.cloud.tencent.com/coding/v3` | `TENCENT_CODING_PLAN_API_KEY` |
| `tencent-token-plan` | openai | `https://api.lkeap.cloud.tencent.com/plan/v3` | `TENCENT_TOKEN_PLAN_API_KEY` |
| `tencent-tokenhub` | openai | `https://tokenhub.tencentmaas.com/v1` | `TENCENT_TOKENHUB_API_KEY` |
| `tensorx` | openai | `https://api.tensorx.ai/v1` | `TENSORX_API_KEY` |
| `the-grid-ai` | openai | `https://api.thegrid.ai/v1` | `THEGRID_API_KEY` |
| `thinkingmachines` | claude | `https://tinker.thinkingmachines.dev/services/tinker-prod/anthropic/api/v1` | `TINKER_API_KEY` |
| `tinfoil` | openai | `https://inference.tinfoil.sh/v1` | `TINFOIL_API_KEY` |
| `together` | openai | `https://api.together.xyz/v1` | — |
| `tokengo` | openai | `https://api.tokengo.com/v1` | `TOKENGO_API_KEY` |
| `tokenrouter` | openai | `https://api.tokenrouter.com/v1` | `TOKENROUTER_API_KEY` |
| `trustedrouter` | openai | `https://api.trustedrouter.com/v1` | `TRUSTEDROUTER_API_KEY` |
| `umans-ai` | openai | `https://api.code.umans.ai/v1` | `UMANS_AI_API_KEY` |
| `umans-ai-coding-plan` | openai | `https://api.code.umans.ai/v1` | `UMANS_AI_CODING_PLAN_API_KEY` |
| `unorouter` | openai | `https://api.unorouter.com/v1` | `UNOROUTER_API_KEY` |
| `upstage` | openai | `https://api.upstage.ai/v1/solar` | — |
| `vancine` | openai | `https://vancine.com/v1` | `VANCINE_API_KEY` |
| `vispark` | openai | `https://api.lab.vispark.in/v1` | `VISPARK_LAB_API_KEY` |
| `vivgrid` | openai | `https://api.vivgrid.com/v1` | `VIVGRID_API_KEY` |
| `volcengine` | openai | `https://ark.cn-beijing.volces.com/api/v3` | `ARK_API_KEY` |
| `volcengine-coding-plan` | openai | `https://ark.cn-beijing.volces.com/api/coding/v3` | `ARK_CODING_PLAN_API_KEY` |
| `voyage` | openai | `https://api.voyageai.com/v1` | — |
| `vultr` | openai | `https://api.vultrinference.com/v1` | `VULTR_API_KEY` |
| `wafer.ai` | openai | `https://pass.wafer.ai/v1` | `WAFER_API_KEY` |
| `wallaby` | openai | `https://api.wallabytoken.com/v1` | `WALLABY_API_KEY` |
| `wally` | openai | `https://inference.runanywhere.ai/v1` | `WALLY_API_KEY` |
| `wandb` | openai | `https://api.inference.wandb.ai/v1` | `WANDB_API_KEY` |
| `writer` | openai | `https://api.writer.com/v1` | — |
| `xai` | openai | `https://api.x.ai/v1` | — |
| `xiaomi` | openai | `https://api.xiaomimimo.com/v1` | `XIAOMI_API_KEY` |
| `xiaomi-token-plan-ams` | openai | `https://token-plan-ams.xiaomimimo.com/v1` | `XIAOMI_API_KEY` |
| `xiaomi-token-plan-cn` | openai | `https://token-plan-cn.xiaomimimo.com/v1` | `XIAOMI_API_KEY` |
| `xiaomi-token-plan-sgp` | openai | `https://token-plan-sgp.xiaomimimo.com/v1` | `XIAOMI_API_KEY` |
| `xpersona` | openai | `https://www.xpersona.co/v1` | `XPERSONA_API_KEY` |
| `yi` | openai | `https://api.lingyiwanwu.com/v1` | — |
| `zai` | openai | `https://api.z.ai/api/paas/v4` | `ZHIPU_API_KEY` |
| `zai-coding-plan` | openai | `https://api.z.ai/api/coding/paas/v4` | `ZHIPU_API_KEY` |
| `zeldoc` | openai | `https://api.zeldoc.ai/v1` | `ZELDOC_API_KEY` |
| `zenifra` | openai | `https://ai.zenifra.com/v1` | `ZENIFRA_AI_KEY` |
| `zenmux` | openai | `https://zenmux.ai/api/v1` | `ZENMUX_API_KEY` |
| `zhipu` | openai | `https://open.bigmodel.cn/api/paas/v4` | — |
| `zhipuai` | openai | `https://open.bigmodel.cn/api/paas/v4` | `ZHIPU_API_KEY` |
| `zhipuai-coding-plan` | openai | `https://open.bigmodel.cn/api/coding/paas/v4` | `ZHIPU_API_KEY` |

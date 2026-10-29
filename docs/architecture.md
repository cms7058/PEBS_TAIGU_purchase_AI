# V0.7技术实现

## 运行结构

Go单体、编译嵌入的HTML/CSS/JavaScript、SQLite。前端无需Node运行时，核心不依赖CGO。SQLite驱动固定modernc.org/sqlite v1.39.1，由go.sum校验。

10学员+管理员单实例，课堂状态1秒轮询，在线心跳45秒内有效。普通请求短时持有课堂锁；远程AI和PDF转换释放锁，不阻塞其他学员。

## 数据与事务

- classroom：账号哈希、页/模块授权、课件元数据、投影、版本和管理审计快照。
- training_attempts：按学员/模块/尝试编号独立保存案例版本、输入、计算、答案、计时和评分。
- training_events：步骤、导入、计算、草稿、提交、复核，含活动时长和历时快照。
- exam_config / exam_attempts：设置、开考快照、服务端截止、作答和复核历史。
- ai_usage：额度预留、调用模式及token记录。

WAL、外键、busy_timeout；作业与事件同事务写入。旧JSON只在数据库无状态时迁移，此后以数据库为准。备份整个DATA_DIR。

## 权限

PBKDF2-SHA256密码哈希、随机初始密码、改密；HttpOnly/SameSite cookie，云端配置Secure。会话保存在内存，重启重新登录。

首页 / 仅选择身份。/admin 与 /learner 分别为管理员和学员页面；浏览器分别请求 /admin/api/... 与 /learner/api/...，服务端路由至下表接口，并校验会话角色与入口一致。两个入口使用独立名称和路径的会话cookie，可在同一浏览器同时登录、分别退出。课件图片、模板和文件上传同样走入口对应的接口路径。原 /api/... 保留兼容接口及投影使用，不读取两个入口的会话cookie。

学员接口不返回Score、标准答案或评分参考。作业校验本人和模块授权。考试期间暂停新的实训请求和AI提示。报告及评分接口验证管理员。

页授权完整名单替换，关闭涵盖离线学员，图片接口同样检查。投影使用独立令牌，仅能读取当前投影页；管理员创建的播放窗口另有受限翻页权限，只能切换相邻页，不能管理账号、学员或课件授权。旧只读令牌不自动获得控制权限。图片以带令牌请求获取blob，不将令牌放入图片URL。

## 接口

| 路径 | 功能 |
| --- | --- |
| /api/login、/api/logout、/api/password | 登录、退出、改密 |
| /api/admin/learners | 学员列表、新增、资料编辑、启停、单个/批量重置密码 |
| /api/me | 本人开放页面和模块 |
| /api/admin/classroom、/api/admin/actions | 课堂状态、页授权/关闭、实训开放/结束、投影 |
| /api/admin/projection-token、/api/projection | 独立投影凭证、当前画面及受限翻页 |
| /api/slide-images/{page} | 受权限保护的页面图片 |
| /api/admin/courseware | 页面包/PPTX/PDF导入 |
| /api/training/{module}、/{module}/template | 案例与CSV模板 |
| /api/training/{module}/start | 创建、恢复、重做 |
| /api/training/attempts/{id}、/{id}/import | 步骤、草稿、心跳、计算、提交、导入 |
| /api/admin/reports、/api/admin/attempts/{id} | 汇总与作业证据、改分留痕 |
| /api/admin/exam、/api/admin/exam/review | 试卷设置与主观题复核 |
| /api/exam | 学员考试状态、开始、保存和交卷 |
| /api/ai/assist、/api/ai/chat、/api/resources | 实训提示、浮动助手对话与官方体验资源 |

学员管理沿用课堂revision防止旧表单覆盖。停用清除页/模块权限并撤销会话；密码写入成功后才撤销旧会话，明文新密码只在该次管理响应返回，审计不保存密码。最多10个启用账号，停用账号及历史记录保留；重启校验账号与学员的一致性，不再固定要求总账号数为11。

进入实训先GET案例及最近记录，不自动POST创建作业。只有选择开始/继续才请求start；已提交记录须明确选择新一轮（retry=true）或查看回执。草稿继续恢复步骤，新一轮初始化空步骤与答案，旧作业保留。

## 教学模型与评分

cases.go中六个模型可独立核算。Go计算和评分，不信任前端自报得分。rule-v1：步骤20、数据20、计算25、决策15、表达10、效率10；表达的规则特征不代表语义评价，讲师可复核。

15秒活动心跳与服务端事件估算有效时长，超过30秒的断续不作连续累计；AI等待切换活动状态。首次提交评分，重复交卷幂等，重做产生新尝试。统计取最近一次尝试，未提交不计均分。

## 课件和AI

PPTX通过ZIP/XML关系读取真实页序及文本，有文件/解压大小限制。PDF通过可选Poppler渲染，1–200页/10MB源文件/转换超时，图片走鉴权接口。原生动画和视频不支持。

AI使用可配置Chat Completions兼容协议，密钥留在后端；2并发、25秒总预算、默认100次额度可配置。无配置仅标注示例回放。业务文档不能赋予工具执行权限。真实模型待密钥联调。

## 学员工作台与助手

学员端使用固定侧栏与内容区，课件为独立功能页。/api/me附带本人最近任务状态、六模块公开目录、考试中状态与助手模式，不含评分或他人记录。浮动助手独立于业务内容区，导航不销毁表单或对话，退出清空。

助手携带最多8条user/assistant历史；课件上下文由服务端按当前授权构造，在线答复返回前再校验。无配置时使用标明的本地知识库。PDF导入时可执行pdftotext，逐页正文最多8000字，提取文本文件限制4MB；聊天节选最多6页、单页5000字。图片页无正文时提示缺失，不提供OCR。

## Docker

runtime-core为scratch/静态程序/CA证书；runtime-pdf增加Alpine与Poppler。Compose通过RUNTIME_TARGET选择，数据持久化，默认本机绑定，正式环境使用HTTPS反向代理。

尚无Docker运行环境，容器构建、镜像体积和云端验证未完成。二进制体积不等于部署总体积。

## V0.3企业采集与历史复盘

新增enterprise.go、acquisition.go、acquisition_api.go。扩展数据保存在training_attempts现有JSON负载中，无需改写旧记录：软件蓝图版本、三来源原始快照、字段映射、自定义字段、来源口径和场景参数。旧记录没有的数据不会被伪造补齐。

- 来源采集：POST /api/training/attempts/{id}，kind为source-import，支持CSV或扁平JSON（含rows/data/records包装）。每来源1–5000行、最多60列，业务主键必须非空唯一。
- 企业仿真：kind=enterprise-sample，仅加载来源；kind=collect按映射显式生成业务表，缺失关联报错，额外来源键记录提示。
- 配置：kind=configure保存customColumns、parameters、dataSource及草稿rows。字段最多20个，核心键不可覆盖；重新校验后才能计算或提交。参数变更保留已校验数据，清除旧计算结果。
- 模板：GET /api/training/{module}/source-template提供分源格式；标准template支持attempt参数，按本人字段快照导出。
- 历史：GET /api/history、/{id}、/{id}/review，服务端验证本人所有权，review还需submitted；不依赖当前模块授权，不返回评分。考试进行中拒绝访问。
- 助手：/api/ai/chat增加historyAttempt上下文，服务端验证本人已提交记录且复盘已经确认、已有分析后才注入复盘；活跃实训不注入此类建议。前端按历史记录隔离会话显示和模型历史，退出清空。

计算和评分统一使用calculateConfigured及保存参数；旧记录参数缺省时采用原始默认值。源码仍为Go单体，前端无新增运行时依赖。


## V0.4访谈式复盘

新增review.go、review-ui.js及SQLite review_sessions(attempt,payload)表。表与已提交练习关联，保存陈述、结构化整理、来源模式、版本、确认时间、分析、模型名和更新时间。练习快照不变。

GET /api/history/{id}/review返回复盘及apiAvailable；POST同路径携带revision和action：save、summarize、manual-summary、confirm、analyze、export-summary、export-analysis、import-summary、import-analysis。导入为人工提供的JSON，不验证外部模型来源。后端校验本人、submitted、考试状态与版本，禁止确认前分析；修改陈述或重新整理清除确认和分析。获取提示词不触发模型、不消耗额度。

模型传输提取为requestAI / requestAIWithLimit，与助手共用调用额度、2并发、25秒超时和权限复核；复盘允许2400输出token，原助手仍1000，不改变兼容协议字段。模型请求释放课堂锁，返回后再检查考试、权限和复盘版本，防止旧结果覆盖新陈述。结构化JSON解析或字段检查失败不覆盖已保存内容。

分析上下文仅含本人确认摘要、历史数据前12行及总量、字段与参数、结论、来源模式与映射、软件蓝图快照。移除原固定利弊建议生成器，避免把通用模板建议当成基于企业陈述的分析。后台密钥与模型未配置时明确不可自动调用；当前对话通过人工传递JSON试用。

前端语音使用SpeechRecognition/WebkitSpeechRecognition，中文、连续转写、仅最终文本落入可编辑框；离开复盘或注销终止识别。未保存的修改即隐藏建议并禁止后续分析，需保存或重新确认。API调用期间锁定编辑以防响应覆盖本地修改；转写/保存/模型错误不伪造成功状态。浏览器厂商可能处理音频，本应用只保存文本。


## V0.5管理员集中咨询（覆盖V0.4学员直接分析接口）

新增consultations(id,attempt,learner,module,payload)表与consultation_owner索引；review_sessions新增JSON属性ticketId，无需改写历史访谈。提交事务同时写入独立咨询快照和访谈关联，失败回滚。快照包含七项摘要、原始陈述、学员、模块、练习次数、确认/提交时间、状态、版本、管理员草稿、咨询关注点、历次公开回复及事件日志。

- 学员GET/POST /api/history/{attempt}/review：POST只允许save、manual-summary、confirm、submit。未确认不能提交；相同内容重复提交返回原记录；修改后新建快照。GET仅返回本人访谈与publicConsultation白名单字段，不返回管理员草稿、补充问题及日志，旧analysis不再向前端透出。
- 管理员GET /api/admin/consultations：分类列表、六模块目录、学员目录及接口配置状态。GET /{id}：内部详情。POST /{id}：export-prompt、save-draft、consult、publish。每次校验版本，publish才追加公开回复。复制提示词不消耗模型额度。
- consult复用2并发、额度、超时保护，在课堂锁外调用，返回后校验咨询记录版本；结果只写draft，管理员明确publish后学员才可查看。外部ChatGPT结果允许粘贴文本或前端转换analysis JSON。
- /api/ai/chat在historyAttempt范围仅返回本人已有管理员公开回复，或引导提交管理员，不再从旧访谈analysis注入模型上下文。
- 历史详情读入时将缺省/空的rows、steps、metrics、columns、sources及字段集合归一为空数组，仅影响显示，不改写数据。静态资产和HTML均no-store，资源URL附版本参数。


## V0.6报告适配与多轮讨论

- consultation_report.go在保存发布及读取旧回复时构造report-v1：sections/blocks、charts、flows、raw、warnings。按标题和【栏目】进行确定性重排，不生成新业务结论。仅当效率指标栏目明确包含学员陈述的小时基线与目标时绘制对比，注明未验证；其他数量图仅统计报告栏目数。保留原始reply.text和解码后的report.raw。
- consultations现有JSON负载扩展questions、draftQuestionId、drafts；回复增加id、questionId、report。旧回复读取时补稳定legacy-reply编号和结构化视图，已有存储不强制改写。
- POST /api/consultations/{id}/questions为学员专属；验证本人、submitted练习、考试、首条公开回复、文本2–2000字。clientId在同一问题内去重，内容不同的同标识请求拒绝。无问答轮数上限。
- 管理员原POST接口增加preview-report（无写入）、questionId选择；草稿按initial或questionId分开保存。publish只公开指定问题的报告，保留旧回复；有未答问题时不标记全部已完成。模型在途时任何新问题或编辑都会改变revision，旧模型结果不覆盖。
- report-ui.js提供HTML章节卡片、SVG柱状图、语义化流程节点及章节追问入口；所有文本转义，不执行模型HTML或脚本，无新增前端运行时依赖。discussionUI按原始回复和各追问分组展示，草稿仅保存在当前浏览器内存，退出清空。
- 管理员咨询资料携带同一学员同一提交的原始确认内容与已发布问答，不携带他人问题或未发布草稿。后台模型仍需API配置，图文重排本身不调用模型。


## V0.7阅读保护和编辑预览

reply-privacy.js只包装学员的discussion-answer与protectedReply助手消息，水印使用登录者ID和Asia/Shanghai查看时间。事件限制作用于回复及包含回复的选择范围，排除外部输入框。blur/visibilitychange只触发遮罩，不作为真实截图证据；打印CSS隐藏当前含受保护回复的页面内容。纯前端控制可绕过，不构成数据访问控制或DRM。

ReportUI增加editable模式及editorText序列化：管理员修改标题和【栏目】正文，保存到原有草稿接口，再用后端报告适配器重新生成图表与流程。管理员GET/POST详情附draftReport，草稿不包含在公开字段中。图文修改未同步前禁止直接发布；保存后仍需显式publish，旧回复不覆盖。

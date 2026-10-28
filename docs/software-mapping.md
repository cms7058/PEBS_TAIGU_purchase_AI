# 软件模块、采集架构与复盘依据

核查日期：2026-10-27。以下是本系统的教学对应关系，不是原厂界面复刻承诺，也不代表厦门太古正在使用这些软件。来源字段是示例导出别名，不冒充原厂私有字段。

| 实训模块 | 对应软件与功能 | 采集对象与字段口径 | 官方依据与渠道 |
| --- | --- | --- | --- |
| M01 需求预测与采购计划 | IBM Maximo for Aviation：Purchasing / Purchase Requisitions；与库存需求配合 | 维修需求汇总、可用库存/在途、平均消耗和交期，按件号及业务粒度归一 | [Purchasing](https://www.ibm.com/docs/en/maximo-for-aviation/7.6.8?topic=applications-purchasing-module)、[Inventory](https://www.ibm.com/docs/en/masv-and-l/maximo-manage/cd?topic=manage-inventory-module)、[试用渠道](https://www.ibm.com/account/reg/us-en/signup?formid=urx-52271) |
| M02 询价解析与比价 | SAP Ariba：Sourcing / RFQ 与报价评审 | 报价、币种汇率/运费、文件核验结果，按报价候选业务键关联 | [Sourcing](https://help.sap.com/docs/ARIBA_SOURCING)、[官方体验渠道](https://www.sap.com/products/spend-management/direct-procurement-software/trial.html) |
| M03 供应商风险评估 | SAP Ariba：Supplier Lifecycle and Performance / Supplier Risk | 履约/质量统计、采购份额、准入资质，统一供应商与统计窗口 | [Supplier Management](https://www.sap.com/products/spend-management/supplier-management.html)、[官方体验渠道](https://www.sap.com/products/spend-management/direct-procurement-software/trial.html) |
| M04 订单跟踪与交付 | IBM Maximo for Aviation：Purchase Orders / Shipments | 订单行与原承诺、最新物流到货、对应维修需求日期，按订单业务键关联 | [Purchasing与Shipments](https://www.ibm.com/docs/en/maximo-for-aviation/7.6.8?topic=applications-purchasing-module)、[试用渠道](https://www.ibm.com/account/reg/us-en/signup?formid=urx-52271) |
| M05 库存策略优化 | IBM Maximo Manage：Inventory / Reorder Items | 同口径领退料统计、实际交期、库存价值，核对单位和库位 | [Reorder Items](https://www.ibm.com/docs/en/masv-and-l/maximo-manage/cd?topic=replenishment-reordering-inventory-items)、[试用渠道](https://www.ibm.com/account/reg/us-en/signup?formid=urx-52271) |
| M06 AOG紧急采购 | IFS Aviation Maintenance：Supply Chain / Maintenance Material Requisitions | 紧急保障候选、物流与通关、质量/适用性核验，按方案业务键关联 | [Supply Chain官方文档](https://docs.ifs.com/documents/Supply_Chain_ICAM_fixed.pdf)、[预约演示](https://www.ifs.com/en/book-a-demo) |

试用链接只提供渠道。账号、期限、产品版本及上述具体模块是否包含在体验范围内，均需由用户与厂商确认。IFS渠道是演示预约，不宣称存在可自助注册的免费沙箱。

## 企业数据采集

采用通用的MRO、ERP、WMS、SRM、QMS分层，具体企业系统名称与接口仍待确认。当前已实现的获取方式为用户上传系统导出的CSV／JSON快照、粘贴接口返回的扁平记录，以及人工填报。没有企业地址及授权时，不尝试连接生产网络，不伪装为已经打通API。

每个模块提供三份独立来源模板及24行分源仿真数据。来源快照记录系统名称、业务对象、来源主键、采集人、截至时间、字段和原始行。来源主键需要在同一汇总粒度下唯一；重复、缺失匹配不能静默合并。额外来源键保留统计范围提示。

学员可为本人每模块增加最多20个字段，支持文本、数值、百分比、布尔和日期，设置名称、单位、用途及是否必填；标准核心字段不可覆盖。新增字段可从来源映射，或人工填报，随练习快照保存。补充字段不自动改变公式；影响核心计算时须映射到标准输入或调整受支持的场景参数。

支持需求倍率、交付时限、供应商权重及资质有效期、到货缓冲、库存服务系数及持有率、AOG到达时限等模块参数。变更参数清除旧结果，服务端评分按保存参数复算。

## 历史复盘与企业适配

[厦门太古官方业务介绍](https://www.haeco.com/en/about/haeco-group/haeco-xiamen)用于确认公开MRO业务范围，不提供其内部系统架构或私有字段。系统中的适配内容是基于该业务场景提出的建议，并明确标为待验证。

软件利弊与企业适配仅在本人的已提交历史记录中开放：按需求识别、采集校验、场景分析、业务复核、提交归档五个步骤展示优势、局限、建议及本次来源/字段/参数依据。草稿不能开启；学员始终不能查看管理员评分或他人记录。历史查看不依赖模块当前是否开放，考试进行中暂停历史访问。

浮动助手在实训页面会将此类问题引导至历史记录；只有历史上下文经过服务端所有权和提交状态校验后，才提供复盘。原始快照、字段定义及参数随记录保留，新一轮不会覆盖，旧版记录未保存的来源不会被补写成真实采集。

## 正式对接仍需资料

实际系统名称和版本、只读导出/API规范、字段字典、主键与汇总规则、物权/库位/时间窗口、接口授权、刷新周期、责任岗位和审批边界。确认后才能把示例分层与字段别名对齐为企业实际映射。

---
title: 财务现金流小程序实现方案
source_idx: 010
section: SQL / 正文
---

## SQL / 正文

```sql
```

关键业务指标定义业务收款 = 集中收款 + 本地收款业务付款 = 赔款支出 + 批减 + 退代垫 + 续费佣金 + 其他支出业务净收支 = 业务收款 - 业务付款核心分析指标：百元保费净现金流百元保费净现金流（当月）：百元保费净现金流（累计）：项目测试推送人员杨城波何家萌李海棠陈超杨城波，何家萌，李海棠，陈超二、成果交付与实施流程1. 数据准备规范数据来源：财务部门准备数据，保存至指定共享地址：\\10.62.0.56\cxhub-dfs\科技创新部\RPA黑灯工厂专用\财务-现金流格式要求：表格第一行应严格作为列名，禁止合并单元格。系统自动识别月份，文件名及列名需保持统一，避免频繁变更。若无需分公司本部数据，则不要在表格中提供相关数据。2. 定时任务配置本项目需要配置两个定时任务：任务一：财务-现金流-数据整理功能：执行数据加工与数据库导入。执行方式：启用任务，按需配置cron表达式，主脚本为 run_sommary.bat。后置动作：执行完成后自动调用报表推送任务。主处理脚本 (run_sommary.bat)：@echo offchcp 65001 >nulsetlocal enabledelayedexpansion

set "BIN_DIR=%~dp0"set "SUMMARY_DIR=%BIN_DIR%excel-summary"set "EXCEL_OUTPUT=summary.xlsx"set "FULL_EXCEL_PATH=%SUMMARY_DIR%\%EXCEL_OUTPUT%"set "SHARE_PATH=\\10.62.0.56\dfs(g)$\科技创新部\RPA黑灯工厂专用\财务-现金流"set "TABLE_NAME=wuh_dw.fr_fgs_cx_cash_flow"set "COL_MAP=工作日期=workdate,机构=org_name,业务收款=business_receipt,业务付款=business_payment,净业务流量=net_business_flow"echo ============================================================echo Excel Summary and Importecho ============================================================echo.echo [Step 1] Generating Excel summary...cd /d "%SUMMARY_DIR%"excel-summary.exeif errorlevel 1 (echo.echo [ERROR] Excel summary generation failed!pauseexit /b 1)echo.echo [SUCCESS] Excel summary generated: %EXCEL_OUTPUT%echo.if not exist "%FULL_EXCEL_PATH%" (echo [ERROR] Generated file not found: %FULL_EXCEL_PATH%pauseexit /b 1)echo [Step 2] Importing to database...cd /d "%BIN_DIR%"excel-import.exe -share="%SHARE_PATH%" -file="%FULL_EXCEL_PATH%" -table="%TABLE_NAME%" -col_map="%COL_MAP%"echo.echo ============================================================pause任务二：财务-现金流-截图发送功能：推送最终报表。 注意：此任务无需手动启动，由“数据整理”任务自动调用。报表地址：http://22.

21.36.143:8080/hub/decision/view/report?viewlet=%25E8%25B4%25A2%25E5%258A%25A1%252F%25E7%258E%25B0%25E9%2587%2591%25E6%25B5%2581%25E7%259C%258B%25E6%259D%25BF.cpt&ref_t=design&ssoToken=jvuTqRk2JXDzM1bHnN%2BgtQ%2BojzMMFi3E6hvMk6XVfuGniLd8%2BN4W2BBgBpw%2B3y8qijtfXtjoHI7%2BaMUK%2B5pRVxZ8YbLu7tZCTgecN1egybqEe3vaZL8DUDVBd6a2kL9shwPQs5am3poJGEIydVLMnJu2vBWnhx44By4xTQE8c7jsVljwpnV8bTTYsRbcI8oKPv0%2BLTPp%2FijeloJSgQwMdpme%2BG4VQ5L843tN%2Fh9QIPBDblORAphB9NJn%2FDmHWPuxW3ktg9o4ndlrPC%2FgOd3Tgu39DJjFAlguzHfQ5Zl1ny5JFxlOFafEGMiRSHzgRhNriJnKlc82PXjctMC42ahUuQ%3D%3D3. 帆软报表数据集定义报表使用以下SQL查询作为数据源：-- 核心数据集查询（日期参数：${workdate}）SELECT     tm.org_name,    tm.business_receipt,    tm.business_payment,    tm.net_business_flow,    ad.total_business_receipt,    ad.total_net_business_flow,    tm.workdate,    -- 当月百元保费净现金流    CASE         WHEN tm.business_receipt = 0 THEN 0        ELSE ROUND((tm.net_business_flow * 100.0 / tm.business_receipt)::numeric, 1)    END AS current_month_net_income_ratio,    -- 累计百元保费净现金流    CASE         WHEN ad.total_business_receipt = 0 THEN 0        ELSE ROUND((ad.total_net_business_flow * 100.0 / ad.total_business_receipt)::numeric, 1)    END AS accumulated_net_income_ratioFROM wuh_dw.fr_fgs_cx_cash_flow tm

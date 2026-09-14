---
title: CREATE OR REPLACE FUNCTION wuh...
source_idx: 018
split_part: 4/6
source_split: true
---

            SELECT a.xzdl 险种大类,
                   a.tb_credential_type,
                   a.policy_no,
                   a.tb_name,
                   wuh_dw.fun_card_getGender_wang(a.tb_credential_no) AS 投保人性别,
                   CASE a.tb_credential_type
                     WHEN '1' THEN
                       '身份证'
                     WHEN '10' THEN
                       '法人证书\企业代码\营业执照'
                     WHEN '2' THEN
                       '军官证'
                     WHEN '3' THEN
                       '军官退休证'
                     WHEN '4' THEN
                       '护照'
                     WHEN '5' THEN
                       '居民户口薄'
                     WHEN '7' THEN
                       '学生证'
                     WHEN '8' THEN
                       '驾驶证'
                     WHEN '9' THEN
                       '台胞证'
                     WHEN '999' THEN
                       '团体其他'
                     ELSE
                       '其他'
                   END AS 投保人证件类型,
                   wuh_dw.fun_card_code_wang(a.tb_credential_no) AS 投保人证件号,
                   wuh_dw.fun_phone_code_wang(a.tb_mobilephone) AS 投保人电话,
                   p.tb_address AS 投保人地址,
                   a.bb_name AS 被保人,
                   wuh_dw.fun_card_getGender_wang(a.bb_credential_no) 被保人性别,
                   CASE a.tb_credential_type
                     WHEN '1' THEN
                       '身份证'
                     WHEN '10' THEN
                       '法人证书\企业代码\营业执照'
                     WHEN '2' THEN
                       '军官证'
                     WHEN '3' THEN
                       '军官退休证'
                     WHEN '4' THEN
                       '护照'
                     WHEN '5' THEN
                       '居民户口薄'
                     WHEN '7' THEN
                       '学生证'
                     WHEN '8' THEN
                       '驾驶证'
                     WHEN '9' THEN
                       '台胞证'
                     WHEN '999' THEN
                       '团体其他'
                     ELSE
                       '其他'
                   END AS 被保人证件类型,
                   wuh_dw.fun_card_code_wang(a.bb_credential_no) 被保人证件号,
                   wuh_dw.fun_phone_code_wang(a.bb_mobilephone) 被保人电话,
                   p.bb_address 被保人地址,
                   a.sp_name 索赔人,
                   wuh_dw.fun_card_getGender_wang(a.sp_credential_no) 索赔人性别,
                   CASE a.tb_credential_type
                     WHEN '1' THEN
                       '身份证'
                     WHEN '10' THEN
                       '法人证书\企业代码\营业执照'
                     WHEN '2' THEN
                       '军官证'
                     WHEN '3' THEN
                       '军官退休证'
                     WHEN '4' THEN
                       '护照'
                     WHEN '5' THEN
                       '居民户口薄'
                     WHEN '7' THEN
                       '学生证'
                     WHEN '8' THEN
                       '驾驶证'
                     WHEN '9' THEN
                       '台胞证'
                     WHEN '999' THEN
                       '团体其他'
                     ELSE
                       '其他'
                   END AS 索赔人证件类型,
                   wuh_dw.fun_card_code_wang(a.sp_credential_no) 索赔人证件号,
                   wuh_dw.fun_phone_code_wang(a.sp_mobilephone) 索赔人电话,
                 
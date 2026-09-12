<script setup lang="ts">
/**
 * 模板规格子表单(复刻 v2 RackTemplateSpecForm;创建模板"初始版本参数"与
 * 发布新版本两处共用)。字段顺序/分段/数值约束逐项对应 v2。
 */
import type { TemplateSpec } from "@/features/rack/rackShared";

const model = defineModel<TemplateSpec>({ required: true });
</script>

<template>
  <div class="rack-template-spec-form">
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12">
        <el-form-item label="机柜类型">
          <el-input v-model="model.type" />
        </el-form-item>
      </el-col>
      <el-col :xs="24" :sm="12">
        <el-form-item label="厂商">
          <el-input v-model="model.manufacturer" />
        </el-form-item>
      </el-col>
    </el-row>
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12">
        <el-form-item label="型号">
          <el-input v-model="model.modelNumber" />
        </el-form-item>
      </el-col>
      <el-col :xs="24" :sm="12">
        <el-form-item label="U 位">
          <el-input-number
            v-model="model.uHeight"
            :min="1"
            :max="100"
            :step="1"
            step-strictly
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
      </el-col>
    </el-row>
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12">
        <el-form-item label="宽(mm)">
          <el-input-number
            v-model="model.widthMm"
            :min="1"
            :step="10"
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
      </el-col>
      <el-col :xs="24" :sm="12">
        <el-form-item label="深(mm)">
          <el-input-number
            v-model="model.depthMm"
            :min="1"
            :step="10"
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
      </el-col>
    </el-row>
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12">
        <el-form-item label="高(mm)">
          <el-input-number
            v-model="model.heightMm"
            :min="1"
            :step="10"
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
      </el-col>
      <el-col :xs="24" :sm="12">
        <el-form-item label="承重(kg)">
          <el-input-number
            v-model="model.loadCapacityKg"
            :min="0"
            :step="1"
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
      </el-col>
    </el-row>
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12">
        <el-form-item label="双路电源">
          <el-switch v-model="model.dualPower" />
        </el-form-item>
      </el-col>
      <el-col :xs="24" :sm="12">
        <el-form-item label="输入路数">
          <el-input-number
            v-model="model.inputCircuits"
            :min="0"
            :step="1"
            step-strictly
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
      </el-col>
    </el-row>
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12">
        <el-form-item label="额定电压(V)">
          <el-input-number
            v-model="model.ratedVoltage"
            :min="0"
            :step="1"
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
      </el-col>
      <el-col :xs="24" :sm="12">
        <el-form-item label="额定电流(A)">
          <el-input-number
            v-model="model.ratedCurrent"
            :min="0"
            :step="1"
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
      </el-col>
    </el-row>
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12">
        <el-form-item label="额定功率(kW)">
          <el-input-number
            v-model="model.ratedPowerKw"
            :min="0"
            :step="0.1"
            :precision="2"
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
      </el-col>
      <el-col :xs="24" :sm="12">
        <el-form-item label="峰值功率(kW)">
          <el-input-number
            v-model="model.peakPowerKw"
            :min="0"
            :step="0.1"
            :precision="2"
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
      </el-col>
    </el-row>
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12">
        <el-form-item label="PDU 数量">
          <el-input-number
            v-model="model.pduCount"
            :min="0"
            :step="1"
            step-strictly
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
      </el-col>
    </el-row>
    <el-form-item label="版本说明">
      <el-input v-model="model.changeNote" type="textarea" :rows="2" />
    </el-form-item>
  </div>
</template>

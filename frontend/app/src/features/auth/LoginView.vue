<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { useSessionStore } from "@/features/auth/session";
import { ApiError } from "@/api/client";

const router = useRouter();
const session = useSessionStore();

const username = ref("");
const password = ref("");
const captchaCode = ref("");
const captchaImg = ref("");
const captchaId = ref("");
const loading = ref(false);

async function refreshCaptcha() {
  const cap = await session.captcha();
  captchaId.value = cap.id;
  captchaImg.value = cap.image;
  captchaCode.value = "";
}

onMounted(refreshCaptcha);

async function submit() {
  if (!username.value || !password.value || !captchaCode.value) {
    ElMessage.warning("请填写用户名、密码和验证码");
    return;
  }
  loading.value = true;
  try {
    await session.login(username.value, password.value, captchaId.value, captchaCode.value);
    router.push("/");
  } catch (e) {
    if (e instanceof ApiError && e.status === 403) {
      ElMessage.error("账户已停用");
    } else if (e instanceof ApiError) {
      ElMessage.error(e.message || "登录失败");
      captchaCode.value = "";
      await refreshCaptcha();
    }
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="login-wrap">
    <el-card class="login-card">
      <template #header>
        <div class="login-brand">
          <span class="brand-mark">ALE</span>
          <span class="brand-title">机柜管理</span>
        </div>
      </template>
      <el-form label-position="top" @submit.prevent="submit">
        <el-form-item label="用户名">
          <el-input v-model="username" placeholder="请输入用户名" data-test="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="password"
            type="password"
            placeholder="请输入密码"
            show-password
            data-test="password"
          />
        </el-form-item>
        <el-form-item label="验证码">
          <div class="captcha-row">
            <el-input v-model="captchaCode" placeholder="验证码" data-test="captcha-input" />
            <img
              v-if="captchaImg"
              :src="captchaImg"
              alt="验证码"
              class="captcha-img"
              title="点击刷新"
              @click="refreshCaptcha"
            />
          </div>
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading" data-test="login-btn">
          登录
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.login-wrap {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--ale-primary-deep), var(--ale-primary));
}
.login-card {
  width: 380px;
  border-radius: var(--ale-radius-lg);
}
.login-brand {
  display: flex;
  align-items: baseline;
  gap: var(--ale-space-2);
}
.brand-mark {
  font-size: 22px;
  font-weight: 700;
  color: var(--ale-primary);
  letter-spacing: 1px;
}
.brand-title {
  font-size: 16px;
  color: var(--ale-ink-700);
}
.captcha-row {
  display: flex;
  gap: var(--ale-space-2);
  width: 100%;
}
.captcha-img {
  height: 32px;
  cursor: pointer;
  border-radius: var(--ale-radius);
  border: 1px solid var(--ale-line);
}
</style>

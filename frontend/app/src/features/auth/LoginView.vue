<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { useSessionStore } from "@/features/auth/session";
import { ApiError } from "@/api/client";
import { reportError } from "@/api/errors";

const router = useRouter();
const session = useSessionStore();

const username = ref("");
const password = ref("");
const captchaCode = ref("");
const captchaImg = ref("");
const captchaId = ref("");
const loading = ref(false);
const captchaError = ref(false);

async function refreshCaptcha() {
  captchaError.value = false;
  try {
    const cap = await session.captcha();
    captchaId.value = cap.id ?? "";
    captchaImg.value = cap.image ?? "";
    captchaCode.value = "";
  } catch (e) {
    // 初始加载失败:显式错误态 + 重试(P1-R06),不再让异常静默逃逸
    captchaError.value = true;
    reportError({
      message: `captcha load failed: ${e instanceof Error ? e.message : String(e)}`,
      role: "anonymous",
    });
  }
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
  <div class="login-wrap login-page">
    <el-card class="login-card">
      <template #header>
        <!-- v2 口径:官方彩色 logo(ale-theme.css .brand::before)置产品名上方,品牌在前 -->
        <div class="brand">
          <h1>机柜管理工具</h1>
          <p>基础资源管理平台</p>
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
          <div v-if="captchaError" class="captcha-error" data-test="captcha-error">
            <span>验证码加载失败</span>
            <el-button
              size="small"
              type="primary"
              plain
              data-test="captcha-retry"
              @click="refreshCaptcha"
            >
              重试
            </el-button>
          </div>
          <div v-else class="captcha-row">
            <el-input v-model="captchaCode" placeholder="验证码" data-test="captcha-input" />
            <img
              v-if="captchaImg"
              :src="captchaImg"
              alt="验证码"
              class="captcha-img"
              title="点击刷新"
              data-test="captcha-img"
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
}
.login-card {
  width: 380px;
  border-radius: var(--ale-radius-lg, 12px);
}
.brand {
  text-align: center;
}
.brand h1 {
  font-size: 20px;
  font-weight: 700;
  margin: 0 0 4px;
}
.brand p {
  font-size: 13px;
  margin: 0;
}
.captcha-row {
  display: flex;
  gap: var(--ale-space-2);
  width: 100%;
}
.captcha-error {
  display: flex;
  align-items: center;
  gap: var(--ale-space-2);
  color: var(--el-text-color-regular, #4b4d50);
}
.captcha-img {
  height: 32px;
  cursor: pointer;
  border-radius: var(--ale-radius, 8px);
  border: 1px solid var(--el-border-color, #d9d9d6);
}
</style>

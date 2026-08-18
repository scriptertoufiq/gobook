import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import { RequireAuth } from './auth/RequireAuth'
import { Layout, PageBody } from './components/Layout'
import { ForgotPassword } from './pages/ForgotPassword'
import { Home } from './pages/Home'
import { Login } from './pages/Login'
import { Profile } from './pages/Profile'
import { Register } from './pages/Register'
import { ResetPassword } from './pages/ResetPassword'
import { Users } from './pages/Users'

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          {/* Public */}
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/forgot-password" element={<ForgotPassword />} />
          {/* Target of the emailed link — set AUTH_PASSWORD_RESET_URL to this route. */}
          <Route path="/reset-password" element={<ResetPassword />} />

          {/* Signed in */}
          <Route element={<RequireAuth />}>
            <Route element={<Layout />}>
              {/* The feed is what a signed-in user lands on. */}
              <Route index element={<Home />} />

              <Route
                path="/account"
                element={
                  <PageBody>
                    <Profile />
                  </PageBody>
                }
              />

              {/* Admin only, mirroring the guard on the Go route table. */}
              <Route element={<RequireAuth role="admin" />}>
                <Route
                  path="/users"
                  element={
                    <PageBody>
                      <Users />
                    </PageBody>
                  }
                />
              </Route>
            </Route>
          </Route>

          <Route
            path="*"
            element={
              <div className="flex min-h-screen items-center justify-center text-slate-500">
                Page not found.
              </div>
            }
          />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}

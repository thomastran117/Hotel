import { useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import api from "@axios";
import useAuthStore from "@stores/authStore";

const ROLES = ["guest", "staff", "admin"];
const ID_TYPES = ["SSN", "SIN", "driving_licence"];

const baseForm = {
  email: "",
  password: "",
  role: "guest",
  first_name: "",
  last_name: "",
  street: "",
  city: "",
  state: "",
  country: "",
  zip: "",
  // guest
  id_type: "SSN",
  id_number: "",
  // staff / admin
  ssn: "",
};

export default function SignupPage() {
  const navigate = useNavigate();
  const setAccessToken = useAuthStore((s) => s.setAccessToken);

  const [form, setForm] = useState(baseForm);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(false);

  const handleChange = (e) =>
    setForm((f) => ({ ...f, [e.target.name]: e.target.value }));

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const { data } = await api.post("/auth/signup", form);
      setAccessToken(data.access_token);
      navigate("/");
    } catch (err) {
      setError(err.response?.data?.error ?? "Signup failed. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  const isGuest = form.role === "guest";

  return (
    <div className="flex items-center justify-center min-h-full py-16 px-4">
      <div className="w-full max-w-lg">
        <h1 className="text-2xl font-semibold mb-6">Create account</h1>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          {/* Account */}
          <Section title="Account">
            <Field label="Email" name="email" type="email" value={form.email} onChange={handleChange} />
            <Field label="Password" name="password" type="password" value={form.password} onChange={handleChange} />
            <div className="flex flex-col gap-1">
              <label className="text-sm text-gray-700">Role</label>
              <select
                name="role"
                value={form.role}
                onChange={handleChange}
                className="border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:border-gray-500"
              >
                {ROLES.map((r) => (
                  <option key={r} value={r}>
                    {r.charAt(0).toUpperCase() + r.slice(1)}
                  </option>
                ))}
              </select>
            </div>
          </Section>

          {/* Profile */}
          <Section title="Profile">
            <div className="grid grid-cols-2 gap-4">
              <Field label="First name" name="first_name" value={form.first_name} onChange={handleChange} />
              <Field label="Last name" name="last_name" value={form.last_name} onChange={handleChange} />
            </div>
            <Field label="Street" name="street" value={form.street} onChange={handleChange} />
            <div className="grid grid-cols-2 gap-4">
              <Field label="City" name="city" value={form.city} onChange={handleChange} />
              <Field label="State" name="state" value={form.state} onChange={handleChange} />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <Field label="Country" name="country" value={form.country} onChange={handleChange} />
              <Field label="ZIP" name="zip" value={form.zip} onChange={handleChange} />
            </div>
          </Section>

          {/* Role-specific */}
          {isGuest ? (
            <Section title="Identity">
              <div className="flex flex-col gap-1">
                <label className="text-sm text-gray-700">ID type</label>
                <select
                  name="id_type"
                  value={form.id_type}
                  onChange={handleChange}
                  className="border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:border-gray-500"
                >
                  {ID_TYPES.map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                </select>
              </div>
              <Field label="ID number" name="id_number" value={form.id_number} onChange={handleChange} />
            </Section>
          ) : (
            <Section title="Identity">
              <Field label="SSN" name="ssn" value={form.ssn} onChange={handleChange} />
            </Section>
          )}

          {error && <p className="text-sm text-red-500">{error}</p>}

          <button
            type="submit"
            disabled={loading}
            className="mt-2 bg-gray-900 text-white text-sm py-2 rounded hover:bg-gray-700 transition-colors disabled:opacity-50"
          >
            {loading ? "Creating account…" : "Create account"}
          </button>
        </form>

        <p className="mt-4 text-sm text-gray-500">
          Already have an account?{" "}
          <Link to="/login" className="text-gray-900 underline">
            Sign in
          </Link>
        </p>
      </div>
    </div>
  );
}

function Section({ title, children }) {
  return (
    <fieldset className="flex flex-col gap-3">
      <legend className="text-xs font-medium text-gray-400 uppercase tracking-wide mb-1">
        {title}
      </legend>
      {children}
    </fieldset>
  );
}

function Field({ label, name, type = "text", value, onChange }) {
  return (
    <div className="flex flex-col gap-1">
      <label className="text-sm text-gray-700">{label}</label>
      <input
        name={name}
        type={type}
        value={value}
        onChange={onChange}
        required
        className="border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:border-gray-500"
      />
    </div>
  );
}

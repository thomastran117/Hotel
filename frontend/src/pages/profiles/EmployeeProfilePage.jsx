import { useEffect, useState } from "react";
import api from "@axios";

export default function EmployeeProfilePage() {
  const [employee, setEmployee] = useState(null);
  const [hotels, setHotels] = useState([]);
  const [positions, setPositions] = useState([]);
  const [form, setForm] = useState({ hotel_id: "", position: "" });
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    Promise.all([
      api.get("/employees/me"),
      api.get("/hotels"),
    ]).then(([empRes, hotelsRes]) => {
      setEmployee(empRes.data);
      setPositions(empRes.data.positions ?? []);
      setHotels(hotelsRes.data ?? []);
    });
  }, []);

  const addPosition = () => {
    if (!form.hotel_id || !form.position.trim()) return;
    const hotelId = Number(form.hotel_id);
    if (positions.some((p) => p.hotel_id === hotelId)) return;
    setPositions((prev) => [
      ...prev,
      { hotel_id: hotelId, position: form.position.trim() },
    ]);
    setForm({ hotel_id: "", position: "" });
  };

  const removePosition = (hotelId) =>
    setPositions((prev) => prev.filter((p) => p.hotel_id !== hotelId));

  const save = async () => {
    setError(null);
    setSaving(true);
    try {
      const { data } = await api.put("/employees/me/positions", positions);
      setEmployee(data);
      setPositions(data.positions ?? []);
    } catch (err) {
      setError(err.response?.data?.error ?? "Failed to save positions.");
    } finally {
      setSaving(false);
    }
  };

  const hotelName = (id) => hotels.find((h) => h.id === id)?.name ?? `Hotel #${id}`;

  const availableHotels = hotels.filter(
    (h) => !positions.some((p) => p.hotel_id === h.id)
  );

  if (!employee) {
    return <p className="p-8 text-sm text-gray-500">Loading…</p>;
  }

  return (
    <div className="max-w-lg mx-auto px-4 py-12">
      <h1 className="text-2xl font-semibold mb-1">
        {employee.first_name} {employee.last_name}
      </h1>
      <p className="text-sm text-gray-500 mb-8">{employee.email}</p>

      <section>
        <h2 className="text-sm font-medium text-gray-400 uppercase tracking-wide mb-4">
          Hotel positions
        </h2>

        {positions.length === 0 ? (
          <p className="text-sm text-gray-400 mb-4">No positions linked yet.</p>
        ) : (
          <ul className="flex flex-col gap-2 mb-4">
            {positions.map((p) => (
              <li
                key={p.hotel_id}
                className="flex items-center justify-between border border-gray-200 rounded px-3 py-2 text-sm"
              >
                <span>
                  <span className="font-medium">{hotelName(p.hotel_id)}</span>
                  <span className="text-gray-400 ml-2">— {p.position}</span>
                </span>
                <button
                  onClick={() => removePosition(p.hotel_id)}
                  className="text-gray-400 hover:text-red-500 transition-colors text-xs"
                >
                  Remove
                </button>
              </li>
            ))}
          </ul>
        )}

        {/* Add position */}
        <div className="flex gap-2">
          <select
            value={form.hotel_id}
            onChange={(e) => setForm((f) => ({ ...f, hotel_id: e.target.value }))}
            className="border border-gray-300 rounded px-3 py-2 text-sm flex-1 focus:outline-none focus:border-gray-500"
          >
            <option value="">Select hotel…</option>
            {availableHotels.map((h) => (
              <option key={h.id} value={h.id}>
                {h.name}
              </option>
            ))}
          </select>
          <input
            type="text"
            placeholder="Position title"
            value={form.position}
            onChange={(e) => setForm((f) => ({ ...f, position: e.target.value }))}
            className="border border-gray-300 rounded px-3 py-2 text-sm flex-1 focus:outline-none focus:border-gray-500"
          />
          <button
            onClick={addPosition}
            disabled={!form.hotel_id || !form.position.trim()}
            className="px-3 py-2 text-sm border border-gray-300 rounded hover:bg-gray-100 transition-colors disabled:opacity-40"
          >
            Add
          </button>
        </div>

        {error && <p className="mt-3 text-sm text-red-500">{error}</p>}

        <button
          onClick={save}
          disabled={saving}
          className="mt-4 w-full bg-gray-900 text-white text-sm py-2 rounded hover:bg-gray-700 transition-colors disabled:opacity-50"
        >
          {saving ? "Saving…" : "Save positions"}
        </button>
      </section>
    </div>
  );
}

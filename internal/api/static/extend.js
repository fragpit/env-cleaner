function extend(period) {
  var buttons = document.querySelectorAll(".btn");
  buttons.forEach(function (btn) {
    btn.disabled = true;
  });

  fetch("/api/environments/" + encodeURIComponent(envID) + "/extend", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      period: period,
      token: token
    })
  })
    .then(function (resp) {
      return resp.json().then(function (data) {
        return { ok: resp.ok, data: data };
      });
    })
    .then(function (result) {
      var actions = document.getElementById("actions");
      var resultDiv = document.getElementById("result");

      actions.classList.add("hidden");
      resultDiv.classList.remove("hidden");

      if (result.ok && result.data.success) {
        resultDiv.className = "result-success";
        resultDiv.innerHTML =
          "Environment extended by <strong>" +
          period +
          "</strong>.<br>New deletion date: <strong>" +
          result.data.data.delete_at +
          "</strong>";
      } else {
        resultDiv.className = "result-error";
        var msg = "Failed to extend";
        if (result.data.error) {
          msg = result.data.error.message || msg;
        }
        resultDiv.textContent = msg;
        buttons.forEach(function (btn) {
          btn.disabled = false;
        });
        actions.classList.remove("hidden");
      }
    })
    .catch(function () {
      var resultDiv = document.getElementById("result");
      resultDiv.classList.remove("hidden");
      resultDiv.className = "result-error";
      resultDiv.textContent = "Network error. Please try again.";
      buttons.forEach(function (btn) {
        btn.disabled = false;
      });
    });
}

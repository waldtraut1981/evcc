export default {
  data: function () {
    return {
      fmtLimit: 100,
      fmtDigits: 1,
    };
  },
  methods: {
    round: function (num, precision) {
      var base = 10 ** precision;
      return (Math.round(num * base) / base).toFixed(precision);
    },
    fmt: function (val) {
      if (val === undefined || val === null) {
        return 0;
      }
      val = Math.abs(val);
      return val >= this.fmtLimit ? this.round(val / 1e3, this.fmtDigits) : this.round(val, 0);
    },
    fmtKw: function (watt, kw = true, withUnit = true) {
      const digits = kw ? 1 : 0;
      const value = kw ? watt / 1000 : watt;
      let unit = "";
      if (withUnit) {
        unit = kw ? " kW" : " W";
      }
      return (
        this.$n(value, { minimumFractionDigits: digits, maximumFractionDigits: digits }) + unit
      );
    },
    fmtUnit: function (val) {
      return Math.abs(val) >= this.fmtLimit ? "k" : "";
    },
    fmtDuration: function (d) {
      if (d <= 0 || d == null) {
        return "—";
      }
      var seconds = "0" + (d % 60);
      var minutes = "0" + (Math.floor(d / 60) % 60);
      var hours = "" + Math.floor(d / 3600);
      if (hours.length < 2) {
        hours = "0" + hours;
      }
      return hours + ":" + minutes.substr(-2) + ":" + seconds.substr(-2);
    },
    fmtShortDuration: function (duration = 0, withUnit = false) {
      if (duration <= 0) {
        return "—";
      }
      var seconds = duration % 60;
      var minutes = Math.floor(duration / 60) % 60;
      var hours = Math.floor(duration / 3600);
      var result = "";
      if (hours >= 1) {
        result = hours + ":" + `${minutes}`.padStart(2, "0");
      } else if (minutes >= 1) {
        result = minutes + ":" + `${seconds}`.padStart(2, "0");
      } else {
        result = `${seconds}`;
      }
      if (withUnit) {
        result += this.fmtShortDurationUnit(duration);
      }
      return result;
    },
    fmtShortDurationUnit: function (duration = 0) {
      if (duration <= 0) {
        return "";
      }
      var minutes = Math.floor(duration / 60) % 60;
      var hours = Math.floor(duration / 3600);
      if (hours >= 1) {
        return "h";
      }
      if (minutes >= 1) {
        return "m";
      }
      return "s";
    },
    fmtDayString: function (date) {
      const YY = `${date.getFullYear()}`;
      const MM = `${date.getMonth() + 1}`.padStart(2, "0");
      const DD = `${date.getDate()}`.padStart(2, "0");
      return `${YY}-${MM}-${DD}`;
    },
    fmtTimeString: function (date) {
      const HH = `${date.getHours()}`.padStart(2, "0");
      const mm = `${date.getMinutes()}`.padStart(2, "0");
      return `${HH}:${mm}`;
    },
    fmtAbsoluteDate: function (date) {
      return new Intl.DateTimeFormat(this.$i18n.locale, {
        weekday: "short",
        hour: "numeric",
        minute: "numeric",
      }).format(date);
    },
    fmtMoney: function (amout = 0, currency = "EUR") {
      return this.$n(amout, { style: "currency", currency });
    },
    fmtPricePerKWh: function (amout = 0, currency = "EUR") {
      let unit = currency;
      let value = amout;
      let maximumFractionDigits = 3;
      if (["EUR", "USD"].includes(currency)) {
        value *= 100;
        unit = "ct";
        maximumFractionDigits = 1;
      }
      return `${this.$n(value, { style: "decimal", maximumFractionDigits })} ${unit}/kWh`;
    },
    fmtTimeAgo: function (elapsed) {
      const units = {
        day: 24 * 60 * 60 * 1000,
        hour: 60 * 60 * 1000,
        minute: 60 * 1000,
        second: 1000,
      };

      const rtf = new Intl.RelativeTimeFormat(this.$i18n.locale, { numeric: "auto" });

      // "Math.abs" accounts for both "past" & "future" scenarios
      for (var u in units)
        if (Math.abs(elapsed) > units[u] || u == "second")
          return rtf.format(Math.round(elapsed / units[u]), u);
    },
  },
};
